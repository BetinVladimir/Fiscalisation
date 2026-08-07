package journal

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDurableHashAckRetention(t *testing.T) {
	p := filepath.Join(t.TempDir(), "journal.sqlite")
	j, e := Open(p)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = j.Append("op1", "RECEIVED", map[string]string{"currency": "EUR"}); e != nil {
		t.Fatal(e)
	}
	if _, e = j.Append("op1", "FISCALIZED", map[string]string{"ref": "r1"}); e != nil {
		t.Fatal(e)
	}
	if e = j.Close(); e != nil {
		t.Fatal(e)
	}
	j2, e := Open(p)
	if e != nil || !j2.Verify() || len(j2.Events()) != 2 {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	if e = j2.Acknowledge(2, now); e != nil {
		t.Fatal(e)
	}
	if len(j2.Eligible(now)) != 0 {
		t.Fatal("deleted before retention")
	}
	if len(j2.Eligible(now.AddDate(0, 4, 0))) != 2 {
		t.Fatal("expected eligible")
	}
	if n, e := j2.Purge(now.AddDate(0, 4, 0)); e != nil || n != 2 {
		t.Fatalf("purge=%d err=%v", n, e)
	}
	if len(j2.Events()) != 0 {
		t.Fatal("eligible acknowledged events not purged")
	}
	if _, e = j2.Append("op2", "RECEIVED", map[string]string{"currency": "EUR"}); e != nil || !j2.Verify() {
		t.Fatalf("append after purge err=%v", e)
	}
}

func TestPurgeNeverCreatesGap(t *testing.T) {
	j, e := Open(filepath.Join(t.TempDir(), "journal.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = j.Append("one", "DONE", map[string]int{"n": 1}); e != nil {
		t.Fatal(e)
	}
	if _, e = j.Append("two", "DONE", map[string]int{"n": 2}); e != nil {
		t.Fatal(e)
	}
	old := time.Now().UTC().AddDate(0, -4, 0)
	if e = j.Acknowledge(1, old); e != nil {
		t.Fatal(e)
	}
	// RetainUntil is based on creation time, therefore neither fresh row is purgeable.
	if n, e := j.Purge(time.Now().UTC()); e != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, e)
	}
	if !j.Verify() {
		t.Fatal("chain invalid")
	}
}
