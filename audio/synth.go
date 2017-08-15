// Copyright (c) 2017 Niko Carpenter
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package audio

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// A Sine generates a sine wave.
type Sine struct {
	AbstractSource
	mtx                      sync.RWMutex // Protects step, iFade, oFade
	step, phase              float64
	iFade, oFade             int
	iFadeBottom, oFadeBottom float64 // 0 = full, >= current amplitude = no fade.
	sampleRate               int
}

// NewSine creates a new sine wave generator.
func NewSine(freq, amplitudeDB float64, sampleRate int) *Sine {
	return &Sine{*NewAbstractSource(amplitudeDB), sync.RWMutex{}, freq / float64(sampleRate), 0, 0, 0, 0, 0, sampleRate}
}

// SetFreq adjusts the frequency.
// The change will take affect at the next call to Read.
func (s *Sine) SetFreq(freq float64) {
	s.setStep(freq / float64(s.sampleRate))
}

func (s *Sine) setStep(step float64) {
	s.mtx.Lock()
	s.step = step
	s.mtx.Unlock()
}

// SetIFade adjusts the fade in in seconds.
// The change will apply only to the next read.
// The buffer size should be >= iFade for the full fade to take effect.
// iFadeBottom sets the starting value (-inf = full fade in, >= current amplitude = no fade).
// -1000 should be low enough to be inaudible
func (s *Sine) SetIFade(iFade, iFadeBottom float64) {
	s.mtx.Lock()
	s.iFade = int(iFade * float64(s.sampleRate))
	s.iFadeBottom = dBFSToLinear(iFadeBottom)
	s.mtx.Unlock()
}

// SetOFade adjusts the fade out in seconds.
// The change will apply only to the next read.
// The buffer size should be >= oFade for the full fade to take effect.
// oFadeBottom sets the ending value (-inf = full fade out, >= current amplitude = no fade).
// -1000 should be low enough to be inaudible
func (s *Sine) SetOFade(oFade, oFadeBottom float64) {
	s.mtx.Lock()
	s.oFade = int(oFade * float64(s.sampleRate))
	s.oFadeBottom = dBFSToLinear(oFadeBottom)
	s.mtx.Unlock()
}

func (s *Sine) Read(buff []float32) (n int, err error) {
	amplitude := s.Amplitude()
	s.mtx.Lock()
	step := s.step
	iFade, oFade := s.iFade, s.oFade
	iFadeBottom, oFadeBottom := s.iFadeBottom, s.oFadeBottom
	s.iFade, s.oFade = 0, 0
	s.iFadeBottom, s.oFadeBottom = 0, 0
	s.mtx.Unlock()

	for i := range buff {
		// Calculate this sample's amplitude from the oscilator amplitude, the iFade and oFade
		sAmp := amplitude
		if i < iFade && sAmp > iFadeBottom {
			sAmp -= (sAmp - iFadeBottom) * float64(iFade-i) / float64(iFade)
		}
		if i >= len(buff)-oFade && sAmp > oFadeBottom {
			sAmp -= (sAmp - oFadeBottom) * float64(oFade+i-len(buff)) / float64(oFade)
		}

		buff[i] = float32(math.Sin(2*math.Pi*s.phase) * sAmp)
		_, s.phase = math.Modf(s.phase + step)
	}
	return len(buff), nil
}

// A WhiteNoise generates white noise.
type WhiteNoise struct {
	AbstractSource
	rnd *rand.Rand
}

// NewWhiteNoise creates a new white noise generator.
func NewWhiteNoise(amplitudeDB float64) *WhiteNoise {
	seed := time.Now().UnixNano()
	return &WhiteNoise{*NewAbstractSource(amplitudeDB),
		rand.New(rand.NewSource(seed))}
}

func (s *WhiteNoise) Read(buff []float32) (n int, err error) {
	amplitude := s.Amplitude()
	for i := range buff {
		buff[i] = s.rnd.Float32() * float32(amplitude)
	}
	return len(buff), nil
}
