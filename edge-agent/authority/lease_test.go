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
