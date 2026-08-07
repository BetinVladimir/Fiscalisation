package journal

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBLERevocationSurvivesRestartAndExpires(t *testing.T) {
	path := filepath.Join(t.TempDir(), "edge.db")
	j, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err = j.RevokeBLESession("session-1", now, now.Add(time.Hour)); err != nil || !j.IsBLESessionRevoked("session-1", now) {
		t.Fatal("revocation not stored", err)
	}
	_ = j.Close()
	j, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if !j.IsBLESessionRevoked("session-1", now.Add(time.Minute)) {
		t.Fatal("revocation lost on restart")
	}
	if n, err := j.PurgeExpiredBLERevocations(now.Add(2 * time.Hour)); err != nil || n != 1 || j.IsBLESessionRevoked("session-1", now.Add(2*time.Hour)) {
		t.Fatalf("expiry purge n=%d err=%v", n, err)
	}
}
