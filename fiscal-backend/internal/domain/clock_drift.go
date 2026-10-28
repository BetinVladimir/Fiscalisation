package domain

import (
	"fmt"
	"time"
)

const (
	// ClockDriftMax is the maximum allowed drift between the fiscal device clock
	// and the host clock. Drift exceeding this blocks financial I/O (Н-18 чл.25 ал.12).
	ClockDriftMax = 2 * time.Hour

	// ClockDriftWarn is the threshold above which drift is flagged as a warning
	// but does not yet block operations.
	ClockDriftWarn = 5 * time.Minute
)

var sofiaLoc *time.Location

func init() {
	var err error
	sofiaLoc, err = time.LoadLocation("Europe/Sofia")
	if err != nil {
		panic("time zone Europe/Sofia not available: " + err.Error())
	}
}

// BusinessDate returns the calendar date of t in the Europe/Sofia timezone.
// All fiscal receipts use Sofia business-date regardless of server UTC offset.
func BusinessDate(t time.Time) time.Time {
	d := t.In(sofiaLoc)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, sofiaLoc)
}

// ErrClockDriftBlock is returned when the device clock drift exceeds ClockDriftMax.
var ErrClockDriftBlock = fmt.Errorf("CLOCK_DRIFT_BLOCK")

// ErrClockDriftWarn is returned when the device clock drift exceeds ClockDriftWarn
// but is still within ClockDriftMax.
var ErrClockDriftWarn = fmt.Errorf("CLOCK_DRIFT_WARN")

// CheckClockDrift returns ErrClockDriftBlock when |deviceTime - hostTime| > 2h,
// ErrClockDriftWarn when > 5min, nil otherwise.
func CheckClockDrift(deviceTime, hostTime time.Time) error {
	drift := deviceTime.Sub(hostTime)
	if drift < 0 {
		drift = -drift
	}
	if drift > ClockDriftMax {
		return ErrClockDriftBlock
	}
	if drift > ClockDriftWarn {
		return ErrClockDriftWarn
	}
	return nil
}
