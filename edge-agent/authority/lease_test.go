package authority

import (
	"testing"
	"time"
)

func TestFenceExpiryExhaustion(t *testing.T) {
	m := New(Lease{FencingToken: 3, OperationFrom: 1, OperationTo: 1, UNPFrom: 7, UNPTo: 7, ExpiresAt: time.Now().Add(time.Hour)})
	if o, u, e := m.Allocate(time.Now(), 3); e != nil || o != 1 || u != 7 {
		t.Fatal(o, u, e)
	}
	if _, _, e := m.Allocate(time.Now(), 3); e == nil {
		t.Fatal("expected exhausted")
	}
	if _, _, e := m.Allocate(time.Now(), 2); e == nil {
		t.Fatal("expected fence rejection")
	}
}

func TestAuthorityHandbackRequiresMonotonicFenceAndRanges(t *testing.T) {
	now := time.Now()
	m := New(Lease{RegisterID: "r", EdgeID: "edge-old", FencingToken: 7, OperationFrom: 10, OperationTo: 19, UNPFrom: 20, UNPTo: 29, ExpiresAt: now.Add(time.Hour)})
	if _, _, err := m.Allocate(now, 7); err != nil {
		t.Fatal(err)
	}
	for name, lease := range map[string]Lease{
		"stale fence":       {FencingToken: 7, OperationFrom: 20, OperationTo: 29, UNPFrom: 30, UNPTo: 39, ExpiresAt: now.Add(time.Hour)},
		"operation overlap": {FencingToken: 8, OperationFrom: 19, OperationTo: 29, UNPFrom: 30, UNPTo: 39, ExpiresAt: now.Add(time.Hour)},
		"unp overlap":       {FencingToken: 8, OperationFrom: 20, OperationTo: 29, UNPFrom: 29, UNPTo: 39, ExpiresAt: now.Add(time.Hour)},
	} {
		if err := m.Install(lease); err == nil {
			t.Fatal(name, "handback accepted")
		}
	}
	m.Revoke()
	next := Lease{RegisterID: "r", EdgeID: "edge-new", FencingToken: 8, OperationFrom: 20, OperationTo: 29, UNPFrom: 30, UNPTo: 39, ExpiresAt: now.Add(2 * time.Hour)}
	if err := m.Install(next); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Allocate(now, 7); err == nil {
		t.Fatal("old fencing token remained valid after handback")
	}
	operation, unp, err := m.Allocate(now, 8)
	if err != nil || operation != 20 || unp != 30 {
		t.Fatal("new authority range not activated", operation, unp, err)
	}
}
