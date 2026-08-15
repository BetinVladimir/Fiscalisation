package domain

import "testing"

func TestTaxGroupLifecycleAndTenantBoundary(t *testing.T) {
	s := NewService("", "test")
	created, err := s.CreateTaxGroup(TaxGroup{TenantID: "tenant-a", Code: "b", Name: "Standard", Rate: "20"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Code != "B" || created.Rate != "20.00" || created.Status != "ACTIVE" || created.Version != 1 {
		t.Fatalf("unexpected created group: %#v", created)
	}
	if _, err = s.CreateTaxGroup(TaxGroup{TenantID: "tenant-a", Code: "B", Name: "Duplicate", Rate: "20"}); err == nil {
		t.Fatal("duplicate tenant tax code must be rejected")
	}
	if got := s.TaxGroupsFor("tenant-b"); len(got) != 0 {
		t.Fatalf("tenant boundary leaked groups: %#v", got)
	}
	updated, err := s.UpdateTaxGroupForTenant(created.ID, created.Version, TaxGroup{Code: "C", Name: "Reduced", Rate: "9,5", Status: "ACTIVE"}, "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Code != "C" || updated.Rate != "9.50" || updated.Version != 2 {
		t.Fatalf("unexpected updated group: %#v", updated)
	}
}
