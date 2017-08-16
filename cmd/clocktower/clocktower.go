// Copyright (c) 2017 Niko Carpenter
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/gordonklaus/portaudio"
	"github.com/n0ot/clocktower"
	"github.com/n0ot/clocktower/audio"
)

func playLive(amplitudeDBFS float64, stopCh <-chan struct{}) {
	portaudio.Initialize()
	defer portaudio.Terminate()

	// Play the live time.
	sampleRate := 44100
	stop := make(chan struct{})
	minutes := clocktower.GetLiveMinutes(stop)
	defer func() {
		close(stop)
	}()
	tas, err := clocktower.NewTimeAudioSource(minutes, amplitudeDBFS, sampleRate)
	if err != nil {
		panic(err)
	}
	s, err := portaudio.OpenDefaultStream(0, 1, float64(sampleRate), 1, audio.Stream(tas))
	if err != nil {
		panic(err)
	}
	defer s.Close()
	if err := s.Start(); err != nil {
		panic(err)
	}
	<-stopCh
	if err := s.Stop(); err != nil {
		panic(err)
	}
}

func main() {
	amplitudeDBFS := flag.Float64("amplitude", -6.0, "Amplitude of output in DBFS. 0 is full volume, -6 is about half, -12 half again, and so on.")
	flag.Parse()

	// Play the live time
	stopCh := make(chan struct{})
	go playLive(*amplitudeDBFS, stopCh)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	<-sigs
	fmt.Println("Done")
	close(stopCh)
}
