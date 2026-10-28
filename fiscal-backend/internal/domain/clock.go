package domain

import "time"

// Clock abstracts the current time so tests can use a fixed time.
type Clock interface {
	Now() time.Time
}

// RealClock returns the actual wall-clock time.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }
