// Copyright (c) 2017 Niko Carpenter
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/n0ot/clocktower"
)

func streamLiveTime(amplitudeDBFS float64, stopCh <-chan struct{}) {
	sampleRate := 44100
    buffSizeMS := 10
	stop := make(chan struct{})
	minutes := clocktower.GetLiveMinutes(stop)
	defer close(stop)

	tas, err := clocktower.NewTimeAudioSource(minutes, amplitudeDBFS, sampleRate)
	if err != nil {
		panic(err)
	}
    buff := make([]float32, int(buffSizeMS * sampleRate / 1000))
	for {
		n, err := tas.Read(buff)
		if err != nil {
			panic(err)
		}
		for i := 0; i < n; i++ {
			if err := binary.Write(os.Stdout, binary.LittleEndian, buff[i]); err != nil {
				panic(err)
			}
		}
	}
	<-stopCh
}

func main() {
	amplitudeDBFS := flag.Float64("amplitude", -6.0, "Amplitude of output in DBFS. 0 is full volume, -6 is about half, -12 half again, and so on.")
	flag.Parse()

	stopCh := make(chan struct{})
	go streamLiveTime(*amplitudeDBFS, stopCh)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	<-sigs
	fmt.Fprintln(os.Stderr, "Done")
	close(stopCh)
}
