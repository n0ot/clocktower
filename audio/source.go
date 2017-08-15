// Copyright (c) 2017 Niko Carpenter
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package audio

import (
	"math"
	"sync"
)

func dBFSToLinear(dBFS float64) float64 {
	return math.Pow(10.0, dBFS/20.0)
}

func fillBuff(buff []float32, val float32, start, end int) {
	for i := start; i < end; i++ {
		buff[i] = val
	}
}

// A Source provides a method, Read,
// which fills a buffer with audio.
// Read returns the number of samples read,
// or an error if audio could not be read.
//
// SetAmpDBFS sets the amplitude of this Source in decibels relative to full scale.
// The amplitude change will take affect when Read is next called.
//
// If 0.0 is passed to SetAmpDBFS, Amplitude will return 1.0 (full volume).
type Source interface {
	Read(buff []float32) (n int, err error)
	SetAmpDBFS(ampDBFS float64)
	Amplitude() float64
}

type AbstractSource struct {
	ampLock   sync.RWMutex // Protects amplitude
	amplitude float64
}

func NewAbstractSource(ampDBFS float64) *AbstractSource {
	return &AbstractSource{sync.RWMutex{}, dBFSToLinear(ampDBFS)}
}

// SetAmpDBFS sets the amplitude of this Source in decibels relative to full scale.
func (s *AbstractSource) SetAmpDBFS(ampDBFS float64) {
	s.ampLock.Lock()
	s.amplitude = dBFSToLinear(ampDBFS)
	s.ampLock.Unlock()
}

// Amplitude gets the maximum amplitude, where 0.0 is silent, and 1.0 is full volume.
func (s *AbstractSource) Amplitude() float64 {
	s.ampLock.RLock()
	amp := s.amplitude
	s.ampLock.RUnlock()
	return amp
}

// A SourceMux mixes multiple Sources into a single Source.
type SourceMux struct {
	AbstractSource
	sources []Source
}

// NewSourceMux creates a new source mux.
// All Sources are mixed with the same amplitude.
// Adjust each source's amplitude individually to mix sources at different volumes.
func NewSourceMux(amplitudeDB float64, sources ...Source) *SourceMux {
	return &SourceMux{*NewAbstractSource(amplitudeDB), sources}
}

func (s *SourceMux) Read(buff []float32) (n int, err error) {
	amplitude := s.Amplitude()
	srcBuff := make([]float32, len(buff))
	// Zero buff, to prevent mixing with the previous buffer.
	fillBuff(buff, float32(0), 0, len(buff))
	for i := range s.sources {
		n, err := s.sources[i].Read(srcBuff)
		if err != nil {
			return 0, err
		}
		// Zero the unwritten portion of the buffer, so that audio isn't mixed twice
		fillBuff(srcBuff, float32(0), n, len(buff))
		for j := range buff {
			buff[j] += srcBuff[j] * float32(amplitude)
		}
	}

	return len(buff), nil
}

// Stream gets a callback function, to be used with libraries like PortAudio.
// The callback function calls source.Read, and panics if there are errors.
// If less than len(buff) samples were read, the remaining samples will be filled with zeros.
func Stream(source Source) func(buff []float32) {
	return func(buff []float32) {
		n, err := source.Read(buff)
		if err != nil {
			panic(err)
		}
		fillBuff(buff, float32(0.0), n, len(buff))
	}
}
