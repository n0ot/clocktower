// Copyright (c) 2017 Niko Carpenter
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package clocktower

import (
	"time"

	"github.com/n0ot/clocktower/audio"
	"github.com/pkg/errors"
)

const (
	tickFade       = 0.0003
	toneFade       = 0.01
	minuteFade     = 0.005
	codeFade       = 0.01
	codeReduceFade = 0.01
)

// mixFrom reads from an audio.Source, mixing the result into the buffer.
func mixFrom(s audio.Source, buff []float32) (n int, err error) {
	nBuff := make([]float32, len(buff))
	n, err = s.Read(nBuff)
	if err != nil {
		return n, err
	}

	for i := 0; i < n && i < len(buff); i++ {
		buff[i] += nBuff[i]
	}

	return len(buff), nil
}

// timeInSamples returns the number of samples in a given time.duration.
// Non integer results will be truncated.
// For example, timeInSamples(2 * time.Second, 44100) = 88200.
func timeInSamples(t time.Duration, sampleRate int) int {
	return int(t) * sampleRate / int(time.Second)
}

// A TimeAudioSource generates audio for a given time.
type TimeAudioSource struct {
	audio.AbstractSource
	// Signals will be encoded to audio from a minute, 1 element of min.bits at a time.
	min     Minute
	minChan <-chan Minute
	// Audio will be generated 1 second at a time.
	secBuff     []float32
	samplesRead int
	sineGen     *audio.Sine
	// Next minute will be announced after 52.5 seconds
	wfa             *WaveFileAnnouncer
	announcerOffset int
}

// NewTimeAudioSource creates a timeAudioSource based on the given time.
// Each minute of time is read from minChan,
// and audio starts at the second in the embedded time.
func NewTimeAudioSource(minChan <-chan Minute, amplitudeDBFS float64, sampleRate int) (*TimeAudioSource, error) {
	secBuff := make([]float32, sampleRate)
	sg := audio.NewSine(440, 0, sampleRate)
	wfa, err := NewWaveFileAnnouncer("announcements", -2.499, sampleRate)
	if err != nil {
		return nil, errors.Wrap(err, "Cannot create WaveFileAnnouncer")
	}
	return &TimeAudioSource{*audio.NewAbstractSource(amplitudeDBFS), Minute{}, minChan, secBuff, 0, sg, wfa, 0}, nil
}

func (s *TimeAudioSource) Read(buff []float32) (n int, err error) {
	amplitude := s.Amplitude()
	secBuff := s.secBuff
	samplesRead := s.samplesRead
	sampleRate := len(secBuff)
	for i := range buff {
		newMinute := false
		if samplesRead == 0 {
			var ok bool
			s.min, ok = <-s.minChan
			if !ok {
				return i, errors.New("No more minutes provided")
			}
			s.wfa.SetTime(s.min.Time.Add(time.Minute))
			s.announcerOffset = 0
			// Seek to the exact time in the minute
			samplesRead += timeInSamples(time.Duration(s.min.Second())*time.Second, sampleRate) +
				timeInSamples(time.Duration(s.min.Nanosecond()), sampleRate)
			newMinute = true
		}
		if samplesRead%sampleRate == 0 || newMinute {
			second := samplesRead / sampleRate
			err := s.nextSecond(second)
			if err != nil {
				return i, errors.Wrapf(err, "Cannot generate audio for %s:%d", s.min.Format("15:04"), second)
			}
		}
		buff[i] = secBuff[samplesRead%sampleRate] * float32(amplitude)
		samplesRead = (samplesRead + 1) % ((s.min.lastSecond + 1) * sampleRate)
	}
	s.samplesRead = samplesRead

	return len(buff), nil
}

// writeMinuteMark fills in the current second with the minute mark.
func (s *TimeAudioSource) writeMinuteMark(second int) error {
	if second != 0 {
		return nil
	}

	freq := float64(1000)
	if s.min.Minute() == 0 {
		freq = 1500
	}
	s.sineGen.SetAmpDBFS(0)
	s.sineGen.SetFreq(freq)
	s.sineGen.SetIFade(minuteFade, -1000)
	s.sineGen.SetOFade(minuteFade, -1000)
	start := 0
	end := timeInSamples(800*time.Millisecond, len(s.secBuff))
	_, err := mixFrom(s.sineGen, s.secBuff[start:end])
	return err
}

// writeTick fills in the current second with the tick, if any.
// A tick may also be inserted at 100 ms for DUT1.
func (s *TimeAudioSource) writeTick(second int) error {
	if second == 0 || second == 29 || second >= 59 {
		return nil // No tick on this second
	}

	freq := float64(1000)
	s.sineGen.SetAmpDBFS(0)
	s.sineGen.SetFreq(freq)
	s.sineGen.SetIFade(tickFade, -1000)
	s.sineGen.SetOFade(tickFade, -1000)
	start := 0
	end := timeInSamples(5*time.Millisecond, len(s.secBuff))
	_, err := mixFrom(s.sineGen, s.secBuff[start:end])
	if err != nil {
		return err
	}

	dut1 := s.min.dut1
	if dut1 == 0 {
		return nil
	}
	dut1I := second
	if dut1 < 0 {
		dut1I -= 8 // 9th second has the first tick for a negative DUT1.
		dut1 *= -1
	}
	if dut1 > 8 {
		dut1 = 8 // Cannot indicate a DUT1 > 0.8 s
	}
	if dut1I <= 0 || dut1 < dut1I {
		return nil
	}

	s.sineGen.SetIFade(tickFade, -1000)
	s.sineGen.SetOFade(tickFade, -1000)
	start = timeInSamples(100*time.Millisecond, len(s.secBuff))
	end = timeInSamples(105*time.Millisecond, len(s.secBuff))
	_, err = mixFrom(s.sineGen, s.secBuff[start:end])
	return err
}

// writeTone fills in the current second with the correct tone, if any.
func (s *TimeAudioSource) writeTone(second int) error {
	if second == 0 || second >= 45 {
		return nil // No tone on this second
	}

	var freq float64
	s.sineGen.SetAmpDBFS(-6)
	s.sineGen.SetIFade(toneFade, -1000)
	s.sineGen.SetOFade(toneFade, -1000)

	if s.min.Minute() == 2 && s.min.Hour() != 0 {
		freq = 440
	} else if s.min.Minute()%2 == 0 {
		freq = 500 // even tone
	} else {
		freq = 600 // Odd tone
	}
	s.sineGen.SetFreq(freq)
	start := timeInSamples(30*time.Millisecond, len(s.secBuff))
	end := timeInSamples(990*time.Millisecond, len(s.secBuff))
	_, err := mixFrom(s.sineGen, s.secBuff[start:end])
	return err
}

// writeTimeCode fills in the current second with its corresponding bit or marker from the time code
func (s *TimeAudioSource) writeTimeCode(second int) error {
	if s.min.bits[second] == bitNone {
		return nil
	}

	freq := float64(100)
	s.sineGen.SetAmpDBFS(-15)
	s.sineGen.SetFreq(freq)
	s.sineGen.SetIFade(codeFade, -1000)
	s.sineGen.SetOFade(codeReduceFade, -30)

	start := timeInSamples(30*time.Millisecond, len(s.secBuff))
	end := timeInSamples(990*time.Millisecond, len(s.secBuff))
	reduceAt := timeInSamples(200*time.Millisecond, len(s.secBuff))
	if s.min.bits[second] == bit1 {
		reduceAt = timeInSamples(500*time.Millisecond, len(s.secBuff))
	} else if s.min.bits[second] == bitMarker {
		reduceAt = timeInSamples(800*time.Millisecond, len(s.secBuff))
	}

	_, err := mixFrom(s.sineGen, s.secBuff[start:reduceAt])
	if err != nil {
		return err
	}

	s.sineGen.SetAmpDBFS(-30)
	s.sineGen.SetOFade(codeFade, -1000)
	_, err = mixFrom(s.sineGen, s.secBuff[reduceAt:end])
	return err
}

// announceNextMinute announces the time at the next tone
func (s *TimeAudioSource) announceNextMinute(second int) error {
	if second < 52 {
		return nil
	}
	start := 0
	// Announcement starts after 52.5 seconds.
	if second == 52 {
		start = timeInSamples(500*time.Millisecond, len(s.secBuff))
	}

	// If the audio is started while the announcement should be playing,
	// seek to the correct point in the announcement.
	skip := timeInSamples(time.Duration(second)*time.Second, len(s.secBuff)) -
		timeInSamples(52500*time.Millisecond, len(s.secBuff)) - s.announcerOffset

	if skip > 0 {
		s.wfa.Skip(skip)
	}

	n, err := mixFrom(s.wfa, s.secBuff[start:])
	s.announcerOffset += n
	return err
}

// nextSecond generates the next second of audio.
func (s *TimeAudioSource) nextSecond(second int) error {
	secBuff := s.secBuff
	// Erase last second's data first
	for i := range secBuff {
		secBuff[i] = float32(0)
	}

	var err error

	err = s.writeMinuteMark(second)
	if err != nil {
		return errors.Wrap(err, "Cannot write minute mark")
	}
	err = s.writeTick(second)
	if err != nil {
		return errors.Wrap(err, "Cannot write tick")
	}
	err = s.writeTone(second)
	if err != nil {
		return errors.Wrap(err, "Cannot write tone")
	}
	err = s.writeTimeCode(second)
	if err != nil {
		return errors.Wrap(err, "Cannot write time code")
	}
	err = s.announceNextMinute(second)
	if err != nil {
		errors.Wrap(err, "Cannot get next minute time announcement.")
	}
	return nil
}
