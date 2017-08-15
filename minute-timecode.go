// Copyright (c) 2017 Niko Carpenter
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package clocktower

import (
	"time"

	"github.com/pkg/errors"
)

var (
	locNewYork    *time.Location // Used to determine Daylight Savings Time status
	minuteEncoder *bCDEncoder
)

func init() {
	var err error
	locNewYork, err = time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}

	minuteEncoder, err = newBCDEncoder([]fieldDef{
		newFieldDef("bit0: minute-marker", 0), // Set to bitNone separately
		newFieldDef("bit1: unused", 0),
		newFieldDef("DST1", 1),
		newFieldDef("LSW", 1), // Leap second at end of month
		newFieldDef("year1s", 1, 2, 4, 8),
		newFieldDef("bit8: unused", 0),
		newFieldDef("P1", 0), // Insert marker separately
		newFieldDef("minute1s", 1, 2, 4, 8, 0),
		newFieldDef("minute10s", 10, 20, 40),
		newFieldDef("bit18: unused", 0),
		newFieldDef("P2", 0), // Insert marker separately
		newFieldDef("hour1s", 1, 2, 4, 8, 0),
		newFieldDef("hour10s", 10, 20),
		newFieldDef("bit27-28: unused", 0, 0),
		newFieldDef("P3", 0), // Insert marker separately
		newFieldDef("dayOfYear1s", 1, 2, 4, 8, 0),
		newFieldDef("dayOfYear10s", 10, 20, 40, 80),
		newFieldDef("P4", 0), // Insert marker separately
		newFieldDef("dayOfYear100s", 100, 200),
		newFieldDef("bit42-48: unused", 0, 0, 0, 0, 0, 0, 0),
		newFieldDef("P5", 0), // Insert marker separately
		newFieldDef("DUT1Sign", 1),
		newFieldDef("year10s", 10, 20, 40, 80),
		newFieldDef("DST2", 1),
		newFieldDef("DUT1Magnitude", 1, 2, 4), // in 100 ms increments
		newFieldDef("P6", 0),                  // Insert marker separately
	})
	if err != nil {
		panic(err)
	}
}

// isDST returns true if Daylight Savings Time is active in New York for the given time.
// This is sufficient to calculate DUT1 and DUT2.
func isDST(t time.Time) bool {
	name, _ := t.In(locNewYork).Zone()
	return name == "EDT"
}

// lastDayInMonth calculates the last day in the given year and month.
func lastDayInMonth(year int, month time.Month) int {
	return time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
}

// lastSecond returns the last second in the minute.
// If lsw is set, and t is the last day of the month at 23:59,
// then lastSecond returns 60. Otherwise, it returns 59.
func lastSecond(t time.Time, lsw bool) int {
	if !lsw {
		return 59
	}
	if t.Hour() != 23 || t.Minute() != 59 {
		return 59
	}
	if t.Day() != lastDayInMonth(t.Year(), t.Month()) {
		return 59
	}
	return 60
}

// Minute holds the time at a given Minute, and the WWV-compatible digital time code.
type Minute struct {
	time.Time
	// bits has length 61, to allow for a leap second.
	bits       [61]byte
	lastSecond int
	lsw        bool // Leap second at end of month
	dut1       int  // Difference between UT1 and UTC, in 100 ms increments.
}

// NewMinute encodes a new minute from the given time.
// The encoded result will be in UTC.
// Set lsw = 1 if a leap second will be inserted at the end of the month.
// DUT1 is the difference between UT1 and UTC, in 100 ms increments.
func NewMinute(t time.Time, lsw, dut1 int) (Minute, error) {
	t = t.UTC() // Don't care about local times
	min := Minute{
		Time: t,
		lsw:  lsw == 1,
		dut1: dut1,
	}
	bits := min.bits[:]

	markers := []int{9, 19, 29, 39, 49, 59} // P1-P6
	bits[0] = bitNone                       // Minute mark
	for _, v := range markers {
		bits[v] = bitMarker
	}

	midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := midnight.AddDate(0, 0, 1)

	dst1 := 0 // DST status at 00:00Z today
	if isDST(midnight) {
		dst1 = 1
	}
	dst2 := 0 // DST status at 24:00Z today
	if isDST(endOfDay) {
		dst2 = 1
	}

	year1s := t.Year() % 10
	year10s := t.Year()%100 - year1s

	minute1s := t.Minute() % 10
	minute10s := t.Minute()%100 - minute1s

	hour1s := t.Hour() % 10
	hour10s := t.Hour()%100 - hour1s

	dayOfYear1s := t.YearDay() % 10
	dayOfYear10s := t.YearDay()%100 - dayOfYear1s
	dayOfYear100s := t.YearDay()%1000 - dayOfYear1s - dayOfYear10s

	dut1Sign, dut1Magnitude := 1, dut1 // dut1Sign is positive
	if dut1 < 0 {
		dut1Sign = 0
		dut1Magnitude *= -1
	}
	if dut1Magnitude > 7 {
		dut1Magnitude = 7 // Only 3 bits for this value.
	}

	err := minuteEncoder.encode(bits, []int{
		0, 0, dst1, lsw, year1s, 0, 0,
		minute1s, minute10s, 0, 0,
		hour1s, hour10s, 0, 0,
		dayOfYear1s, dayOfYear10s, 0,
		dayOfYear100s, 0, 0,
		dut1Sign, year10s, dst2, dut1Magnitude, 0,
	})
	if err != nil {
		return min, errors.Wrapf(err, "Cannot encode minute %s", t.Format("15:04"))
	}

	min.lastSecond = lastSecond(t, min.lsw)

	return min, nil
}
