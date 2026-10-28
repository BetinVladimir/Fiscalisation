package domain

import (
	"testing"
	"time"
)

// TestBusinessDateDSTSpringForwardAllMinutes tests BusinessDate across all 1440 minutes
// of the day when Bulgaria (Europe/Sofia) springs forward (EEST, UTC+3).
// 2024-03-31: at 03:00 local time, clocks spring forward to 04:00 (01:00 UTC).
// For every minute in the 24-hour window starting at midnight Sofia (= 22:00 UTC on 2024-03-30),
// BusinessDate must return the correct Sofia calendar date.
func TestBusinessDateDSTSpringForwardAllMinutes(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Fatalf("Europe/Sofia not available: %v", err)
	}
	// Midnight Sofia on 2024-03-31 = 2024-03-30T22:00:00 UTC
	sofiaDate := time.Date(2024, 3, 31, 0, 0, 0, 0, loc)
	startUTC := sofiaDate.UTC()

	failures := 0
	for m := 0; m < 1440; m++ {
		ts := startUTC.Add(time.Duration(m) * time.Minute)
		got := BusinessDate(ts)
		sofiaTime := ts.In(loc)
		want := time.Date(sofiaTime.Year(), sofiaTime.Month(), sofiaTime.Day(), 0, 0, 0, 0, loc)
		if !got.Equal(want) {
			t.Errorf("minute %d (%v UTC): BusinessDate=%v want %v", m, ts, got, want)
			failures++
			if failures > 5 {
				t.Fatal("too many failures, stopping")
			}
		}
	}
}

// TestBusinessDateDSTFallBackAllMinutes tests all 1440 minutes during the fall-back transition.
// 2024-10-27: at 04:00 local time, clocks fall back to 03:00 (01:00 UTC).
// The ambiguous hour (03:00..04:00 Sofia, UTC 00:00..01:00) must still return the correct date.
func TestBusinessDateDSTFallBackAllMinutes(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Fatalf("Europe/Sofia not available: %v", err)
	}
	// Midnight Sofia on 2024-10-27 = 2024-10-26T21:00:00 UTC (EEST = UTC+3)
	sofiaDate := time.Date(2024, 10, 27, 0, 0, 0, 0, loc)
	startUTC := sofiaDate.UTC()

	failures := 0
	for m := 0; m < 1440; m++ {
		ts := startUTC.Add(time.Duration(m) * time.Minute)
		got := BusinessDate(ts)
		sofiaTime := ts.In(loc)
		want := time.Date(sofiaTime.Year(), sofiaTime.Month(), sofiaTime.Day(), 0, 0, 0, 0, loc)
		if !got.Equal(want) {
			t.Errorf("minute %d (%v UTC): BusinessDate=%v want %v", m, ts, got, want)
			failures++
			if failures > 5 {
				t.Fatal("too many failures, stopping")
			}
		}
	}
}

// TestBusinessDateDSTCrossMidnightSofia verifies timestamps that span midnight Sofia
// during the DST transition return the new calendar day.
func TestBusinessDateDSTCrossMidnightSofia(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Fatalf("Europe/Sofia not available: %v", err)
	}
	cases := []struct {
		utc  time.Time
		wDay int
		wMon time.Month
		wYr  int
	}{
		// Spring forward: 2024-03-30T21:59 UTC = 2024-03-30T23:59 Sofia (one minute before midnight)
		{time.Date(2024, 3, 30, 21, 59, 0, 0, time.UTC), 30, 3, 2024},
		// Spring forward: 2024-03-30T22:00 UTC = 2024-03-31T00:00 Sofia (new day!)
		{time.Date(2024, 3, 30, 22, 0, 0, 0, time.UTC), 31, 3, 2024},
		// Spring-forward hour: 2024-03-31T01:01 UTC = 2024-03-31T04:01 Sofia (skipped 03:xx)
		{time.Date(2024, 3, 31, 1, 1, 0, 0, time.UTC), 31, 3, 2024},
		// Fall-back: 2024-10-26T20:59 UTC = 2024-10-26T23:59 Sofia (one minute before midnight)
		{time.Date(2024, 10, 26, 20, 59, 0, 0, time.UTC), 26, 10, 2024},
		// Fall-back: 2024-10-26T21:00 UTC = 2024-10-27T00:00 Sofia (new day)
		{time.Date(2024, 10, 26, 21, 0, 0, 0, time.UTC), 27, 10, 2024},
		// Fall-back ambiguous: 2024-10-27T00:30 UTC = 2024-10-27T03:30 Sofia (first pass, EEST)
		{time.Date(2024, 10, 27, 0, 30, 0, 0, time.UTC), 27, 10, 2024},
		// Fall-back ambiguous: 2024-10-27T01:30 UTC = 2024-10-27T03:30 Sofia (second pass, EET)
		{time.Date(2024, 10, 27, 1, 30, 0, 0, time.UTC), 27, 10, 2024},
	}
	for _, tc := range cases {
		got := BusinessDate(tc.utc)
		got2 := got.In(loc)
		if got2.Year() != tc.wYr || got2.Month() != tc.wMon || got2.Day() != tc.wDay {
			t.Errorf("BusinessDate(%v)=%v, want %04d-%02d-%02d in Sofia", tc.utc, got, tc.wYr, tc.wMon, tc.wDay)
		}
	}
}

// TestBusinessDateDSTLocationIsSofia ensures BusinessDate always returns a time in Sofia timezone.
func TestBusinessDateDSTLocationIsSofia(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Fatalf("Europe/Sofia not available: %v", err)
	}
	timestamps := []time.Time{
		time.Date(2024, 3, 31, 1, 0, 0, 0, time.UTC),  // spring forward moment
		time.Date(2024, 10, 27, 1, 0, 0, 0, time.UTC), // fall back moment
		time.Date(2024, 7, 15, 12, 0, 0, 0, time.UTC), // summer
		time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC), // winter
	}
	_ = loc // used for context only
	for _, ts := range timestamps {
		bd := BusinessDate(ts)
		// Location must be named "Europe/Sofia" (compare by name, not pointer)
		if bd.Location().String() != "Europe/Sofia" {
			t.Errorf("BusinessDate(%v).Location()=%v, want Europe/Sofia", ts, bd.Location())
		}
		// Business date must be midnight in Sofia
		if bd.Hour() != 0 || bd.Minute() != 0 || bd.Second() != 0 {
			t.Errorf("BusinessDate(%v)=%v is not midnight in Sofia", ts, bd)
		}
	}
}
