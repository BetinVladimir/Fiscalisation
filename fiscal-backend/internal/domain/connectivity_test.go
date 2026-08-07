package domain

import "testing"

func TestConnectivityProbePersistsAndBlocksWhenFiscalDeviceLost(t *testing.T) {
	store := &testStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(repo, NewSimulator(false))
	probe, err := svc.Connectivity("register-1", "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if probe.State != "FAILED" || probe.RecommendedTransport != "BLOCK" || probe.Hops["fiscal_device"]["state"] != "UNAVAILABLE" {
		t.Fatalf("lost fiscal device must block, got %+v", probe)
	}
	repo, err = NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	svc = NewService(repo, NewSimulator(true))
	if _, err = svc.GetConnectivityProbe(probe.ProbeID, "tenant-2"); err == nil {
		t.Fatal("cross-tenant probe leaked")
	}
	got, err := svc.GetConnectivityProbe(probe.ProbeID, "tenant-1")
	if err != nil || got.ProbeID != probe.ProbeID {
		t.Fatal(got, err)
	}
}
