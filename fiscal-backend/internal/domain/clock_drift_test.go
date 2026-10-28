package domain

import (
	"errors"
	"testing"
	"time"
)

func TestClockDriftConstants(t *testing.T) {
	if ClockDriftMax != 2*time.Hour {
		t.Errorf("ClockDriftMax = %v, want 2h", ClockDriftMax)
	}
	if ClockDriftWarn != 5*time.Minute {
		t.Errorf("ClockDriftWarn = %v, want 5m", ClockDriftWarn)
	}
}

func TestBusinessDateUsesEuropeSofia(t *testing.T) {
	// 2024-03-31T01:30:00 UTC = 2024-03-31T03:30:00 Sofia (EEST+2), business date = 2024-03-31
	utc := time.Date(2024, 3, 31, 1, 30, 0, 0, time.UTC)
	bd := BusinessDate(utc)
	if bd.Year() != 2024 || bd.Month() != 3 || bd.Day() != 31 {
		t.Errorf("BusinessDate(%v) = %v, want 2024-03-31", utc, bd)
	}
}

func TestBusinessDateMidnightSofia(t *testing.T) {
	// 2024-01-15T22:00:00 UTC = 2024-01-16T00:00:00 Sofia (EET+2), business date = 2024-01-16
	utc := time.Date(2024, 1, 15, 22, 0, 0, 0, time.UTC)
	bd := BusinessDate(utc)
	if bd.Year() != 2024 || bd.Month() != 1 || bd.Day() != 16 {
		t.Errorf("BusinessDate midnight: got %v, want 2024-01-16", bd)
	}
}

func TestClockDriftOver2hBlocksSales(t *testing.T) {
	host := time.Now().UTC()
	device := host.Add(2*time.Hour + 1*time.Minute) // 2h01m drift
	err := CheckClockDrift(device, host)
	if !errors.Is(err, ErrClockDriftBlock) {
		t.Fatalf("2h01m drift should return ErrClockDriftBlock, got %v", err)
	}
}

func TestClockDrift5minWarnsOnly(t *testing.T) {
	host := time.Now().UTC()
	device := host.Add(6 * time.Minute) // 6min drift
	err := CheckClockDrift(device, host)
	if !errors.Is(err, ErrClockDriftWarn) {
		t.Fatalf("6min drift should return ErrClockDriftWarn, got %v", err)
	}
}

func TestClockDriftUnderWarnThreshold(t *testing.T) {
	host := time.Now().UTC()
	device := host.Add(3 * time.Minute) // 3min drift — no warn
	err := CheckClockDrift(device, host)
	if err != nil {
		t.Fatalf("3min drift should return nil, got %v", err)
	}
}

func TestClockDriftNegativeIsSymmetric(t *testing.T) {
	host := time.Now().UTC()
	device := host.Add(-(2*time.Hour + 1*time.Minute)) // device behind by 2h01m
	err := CheckClockDrift(device, host)
	if !errors.Is(err, ErrClockDriftBlock) {
		t.Fatalf("negative 2h01m drift should return ErrClockDriftBlock, got %v", err)
	}
}
