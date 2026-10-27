package domain

import (
	"testing"
	"time"
)

func TestRealClockReturnsNonZero(t *testing.T) {
	var c Clock = RealClock{}
	now := c.Now()
	if now.IsZero() {
		t.Error("RealClock.Now() returned zero time")
	}
}

func TestClockInterfaceSatisfied(t *testing.T) {
	fixed := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	var _ Clock = RealClock{}
	if fixed.IsZero() {
		t.Error("fixed time is zero")
	}
}
