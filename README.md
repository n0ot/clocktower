Clocktower mimics the time signal, as produced by [WWV](https://www.nist.gov/pml/time-and-frequency-division/radio-stations/wwv).
It produces a tick on every second except :29, :59 and :60 (when there is a leap second).
The time at the next minute tone is announced after 52.5 seconds past every minute.
Like WWV, times are in UTC.
The binary coded decimal encoding was also implemented.
I tried to follow the format as described by [WWV's Wikipedia page](https://en.wikipedia.org/w/index.php?title=WWV_(radio_station)&oldid=794024045), retrieved on August 15, 2017, as best I could. I'd love to hear if I missed anything.

## Differences and TODOs
* There are several minutes during which WWV does not play audio tones, airing announcements in their place.
    As Clocktower does not air announcements, tones are played on all minutes.
* Whether a leap second will be inserted at the end of the month (LSW), and the difference between UT1 and UTC ([DUT1](https://en.wikipedia.org/wiki/DUT1))
    are values which need to be retrieved from the internet. This has not been implemented yet.
    DUT1 has been manually set to 3, to demonstrate how it is encoded; also notice the double ticks on the first 3 seconds.
* In a "production" environment, very low latencies are super critical for these kinds of applications. I tried to get the lowest latency as I could,
    but getting delays down to the nanosecond level seems impossible. Perhaps there is a way to sync the clock with the hardware playback, but that is beyond my knowledge at the moment.

## Cloning
Clocktower contains pre-recorded time announcements stored in wave files.
These files are stored in [Git Large File Storage](https://git-lfs.github.com/), rather than in the repository itself.
To download the wave files when you clone, you first need to install Git-LFS.

## Building
Install the version of [Go](https://golang.org/dl/) for your operating system.

With Go and git-lfs installed, run

    go get -u github.com/n0ot/clocktower

In clocktower/cmd/clocktower:

    go build && go install

The announcements directory must be in the current working directory.

## Usage
Clocktower sends its generated audio to standard output. To play the audio, pipe it to another program that can play it like [SoX](http://sox.sourceforge.net/).
The audio is PCM float 32, 44.1 kHz mono.

Try:

    clocktower | play -t raw -e float -b 32 -r 44100 -c 1 -
    clocktower -amplitude -12 | play -t raw -e float -b 32 -r 44100 -c 1 -  # Quieter, amplitude is in dB.

If you want to encode the audio for streaming, and your encoder does not support floating samples, use SoX first to convert.

    clocktower | \
        sox -t raw -e float -b 32 -r 44100 -c 1 - -t s16 - | \
        opusenc --bitrate 32 --raw --raw-rate 44100 --raw-chan 1 - - | \
        ezstream -c clocktower_opus_stream.xml # Stream online as 32 Kbps mono Opus.

Streaming online will introduce significantly greater delay.
