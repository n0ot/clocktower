// Copyright (c) 2017 Niko Carpenter
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

// Package clocktower provides logic to encode time into
// a WWV-compatible audio signal.
package clocktower

import (
	"log"
	"time"
)

// GetLiveMinutes returns a channel, on which a new Minute based on the current time
// is set at the start of each minute.
// Close the stop channel to stop producing minutes. The minutes channel will be closed.
func GetLiveMinutes(stop <-chan struct{}) <-chan Minute {
	minutes := make(chan Minute)
	go func() {
		// By using a timer instead of a ticker, the beginning of the next minute
		// will still be tracked correctly, even if the time is changed.
		minute, err := getCurrentMinute()
		if err != nil {
			log.Printf("Error getting minute: %v\n", err)
			close(minutes)
			return
		}
		t := time.NewTimer(timeUntilNext(minute))
		for {
			minutes <- minute
			select {
			case <-stop:
				close(minutes)
				// Drain the timer
				if !t.Stop() {
					<-t.C
				}
				log.Printf("No longer getting minutes.\n")
				return
			case <-t.C:
				minute, err = getCurrentMinute()
				if err != nil {
					log.Printf("Error getting minute: %v\n", err)
					close(minutes)
					return
				}
				t.Reset(timeUntilNext(minute))
			}
		}
	}()

	return minutes
}

func timeUntilNext(minute Minute) time.Duration {
	return minute.Truncate(time.Minute).Add(time.Minute).Sub(minute.Time)
}

// getCurrentMinute gets the current minute, also looking up LSW and DUT1.
func getCurrentMinute() (Minute, error) {
	lsw := 0  // TODO
	dut1 := 3 // TODO
	return NewMinute(time.Now(), lsw, dut1)
}
