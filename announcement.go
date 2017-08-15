// Copyright (c) 2017 Niko Carpenter
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package clocktower

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/n0ot/clocktower/audio"
)

// readWaveFile loads a wave file into memory,
// converting each sample to float32.
// TODO: The header is not yet examined; 44.1 KHZ mono is assumed for now.
func readWaveFile(filename string) ([]float32, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	f.Seek(44, io.SeekStart) // Skip over 44 byte header
	fBuff, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	buff := make([]float32, len(fBuff)/2)
	for i := range buff {
		buff[i] = float32(int16(binary.LittleEndian.Uint16(fBuff[i*2:(i+1)*2]))) / float32(0x8000)
	}

	return buff, nil
}

// WaveFileAnnouncer announces the time based on a set of wave files.
// Set the time with SetTime, and read the audio with Read.
// After the entire time announcement has been read, silence will be returned indefinitely.
// Call SetTime again to announce another time.
// Calling SetTime before the current time has been completely read will override the current time announcement.
type WaveFileAnnouncer struct {
	// Holds a copy of the announcer audio.
	audio.AbstractSource
	atTheTone, hour, hours, minute, minutes, utc []float32
	numbers                                      [60][]float32
	timeAnnouncement                             []float32
	offset                                       int
	sampleRate                                   int // TODO: Announcements are always 44100 HZ for now.
}

// NewWaveFileAnnouncer initializes a WaveFileAnnouncer,
// Loading in wave files from dir.
//
// Each wave file must be in 44.1 KHZ mono, and the following files should exist:
//     0-59.wav: Spoken numbers from zero to fifty-nine; used for both hours and minutes.
//     att.wav: "At the tone,"
//     hours.wav: "hours"
//     minutes.wav: "minutes"
//     utc.wav: "Coordinated Universal Time"
func NewWaveFileAnnouncer(dir string, amplitudeDBFS float64, sampleRate int) (*WaveFileAnnouncer, error) {
	wfa := WaveFileAnnouncer{}
	wfa.AbstractSource = *audio.NewAbstractSource(amplitudeDBFS)
	wfa.sampleRate = sampleRate
	var err error

	wfa.atTheTone, err = readWaveFile(path.Join(dir, "att.wav"))
	if err != nil {
		return nil, err
	}
	wfa.hour, err = readWaveFile(path.Join(dir, "hour.wav"))
	if err != nil {
		return nil, err
	}
	wfa.hours, err = readWaveFile(path.Join(dir, "hours.wav"))
	if err != nil {
		return nil, err
	}
	wfa.minute, err = readWaveFile(path.Join(dir, "minute.wav"))
	if err != nil {
		return nil, err
	}
	wfa.minutes, err = readWaveFile(path.Join(dir, "minutes.wav"))
	if err != nil {
		return nil, err
	}
	wfa.utc, err = readWaveFile(path.Join(dir, "utc.wav"))
	if err != nil {
		return nil, err
	}

	for i := 0; i < 60; i++ {
		wfa.numbers[i], err = readWaveFile(path.Join(dir, fmt.Sprintf("%d.wav", i)))
		if err != nil {
			return nil, err
		}
	}

	return &wfa, nil
}

// Read returns the announced time in the format "At the tone, 15 hours, 4 minutes, coordinated universal time."
// Once the current time has been completely read, silence will be returned indefinitely.
func (wfa *WaveFileAnnouncer) Read(buff []float32) (n int, err error) {
	amplitude := wfa.Amplitude()
	for i := range buff {
		if wfa.offset >= len(wfa.timeAnnouncement) {
			// Just fill the buffer with silence
			buff[i] = float32(0)
			continue
		}
		buff[i] = wfa.timeAnnouncement[wfa.offset] * float32(amplitude)
		wfa.offset++
	}

	return len(buff), nil
}

// SetTime sets the time and overrides the previous time announcement.
func (wfa *WaveFileAnnouncer) SetTime(t time.Time) {
	pauseAfterHours := timeInSamples(100*time.Millisecond, wfa.sampleRate) // Pause between rest of time announcement and "Coordinated Universal Time"
	pauseBeforeUTC := timeInSamples(800*time.Millisecond, wfa.sampleRate)  // Pause between rest of time announcement and "Coordinated Universal Time"
	// First calculate the length needed
	hours := wfa.hours
	if t.Hour() == 1 {
		hours = wfa.hour
	}
	minutes := wfa.minutes
	if t.Minute() == 1 {
		minutes = wfa.minute
	}

	length := len(wfa.atTheTone) +
		len(wfa.numbers[t.Hour()]) +
		len(hours) +
		pauseAfterHours +
		len(wfa.numbers[t.Minute()]) +
		len(minutes) +
		pauseBeforeUTC +
		len(wfa.utc)

	wfa.timeAnnouncement = make([]float32, length)

	i := 0
	copy(wfa.timeAnnouncement[i:], wfa.atTheTone)
	i += len(wfa.atTheTone)
	copy(wfa.timeAnnouncement[i:], wfa.numbers[t.Hour()])
	i += len(wfa.numbers[t.Hour()])
	copy(wfa.timeAnnouncement[i:], hours)
	i += len(hours)
	i += pauseAfterHours
	copy(wfa.timeAnnouncement[i:], wfa.numbers[t.Minute()])
	i += len(wfa.numbers[t.Minute()])
	copy(wfa.timeAnnouncement[i:], minutes)
	i += len(minutes)
	i += pauseBeforeUTC
	copy(wfa.timeAnnouncement[i:], wfa.utc)
	i += len(wfa.utc)

	wfa.offset = 0
}

// Skip skips n samples of the announcement.
func (wfa *WaveFileAnnouncer) Skip(n int) {
	wfa.offset += n
}
