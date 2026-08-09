package journal

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// TestAcceleratedSevenDayJournalSoak models a command-result event every ten
// minutes for seven days, with hourly ACKs and a SQLite reopen every day.
func TestAcceleratedSevenDayJournalSoak(t *testing.T) {
	const slots = 7 * 24 * 6
	path := filepath.Join(t.TempDir(), "seven-day-soak.sqlite")
	j, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for slot := 1; slot <= slots; slot++ {
		event, appendErr := j.Append(fmt.Sprintf("seven-day-%04d", slot), "COMMAND_RESULT", map[string]any{"slot": slot, "state": "FISCALIZED"})
		if appendErr != nil || event.Sequence != int64(slot) {
			t.Fatalf("append slot=%d sequence=%d err=%v", slot, event.Sequence, appendErr)
		}
		if slot%6 == 0 {
			if err = j.Acknowledge(int64(slot), time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
		}
		if slot%(24*6) == 0 {
			if err = j.Close(); err != nil {
				t.Fatal(err)
			}
			j, err = Open(path)
			if err != nil || !j.Verify() || len(j.Events()) != slot {
				t.Fatalf("daily restart slot=%d events=%d err=%v", slot, len(j.Events()), err)
			}
		}
	}
	defer j.Close()
	events := j.Events()
	if len(events) != slots || len(j.Unacknowledged(100)) != 0 || !j.Verify() {
		t.Fatalf("journal soak mismatch events=%d pending=%d", len(events), len(j.Unacknowledged(100)))
	}
	if eligible := j.Eligible(events[0].CreatedAt.Add(7 * 24 * time.Hour)); len(eligible) != 0 {
		t.Fatal("seven-day operation violated mandatory three-month retention")
	}
	purgeAt := events[len(events)-1].RetainUntil.Add(time.Nanosecond)
	if purged, purgeErr := j.Purge(purgeAt); purgeErr != nil || purged != slots || len(j.Events()) != 0 || !j.Verify() {
		t.Fatalf("retention purge=%d remaining=%d err=%v", purged, len(j.Events()), purgeErr)
	}
	if event, appendErr := j.Append("post-soak", "COMMAND_RESULT", map[string]any{"state": "FISCALIZED"}); appendErr != nil || event.Sequence != slots+1 || !j.Verify() {
		t.Fatalf("post-purge continuity sequence=%d err=%v", event.Sequence, appendErr)
	}
}
