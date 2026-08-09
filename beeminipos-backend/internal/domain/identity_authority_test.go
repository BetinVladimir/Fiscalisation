package domain

import (
	"testing"
	"time"
)

func TestIdentityBindingAndSessionRequireActiveEmployeeAtCommit(t *testing.T) {
	s := NewService("http://invalid", "2026-08-07")
	employee, err := s.CreateEmployee(Employee{TenantID: "tenant-a", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001", Roles: []string{"CASHIER"}, Status: "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	inactive, err := s.UpdateEmployee(employee.ID, employee.Version, Employee{TenantID: "tenant-a", FirstName: employee.FirstName, LastName: employee.LastName, OperatorCode: employee.OperatorCode, Roles: employee.Roles, Status: "INACTIVE"})
	if err != nil || inactive.Active {
		t.Fatalf("employee deactivation failed: %#v %v", inactive, err)
	}
	if _, err = s.BindEmployeeIdentity("tenant-a", employee.ID, "cashier-1", "https://identity.example.test"); err == nil {
		t.Fatal("inactive employee received immutable identity binding")
	}
	if _, err = s.RegisterOperatorSession("tenant-a", employee.ID, "00000000-0000-4000-8000-000000000001", "token-fingerprint", time.Now().Add(time.Hour)); err == nil {
		t.Fatal("inactive employee received active operator session")
	}
}

func TestCreateRespectsInactiveProductAndEmployeeStatus(t *testing.T) {
	s := NewService("http://invalid", "2026-08-07")
	product, err := s.CreateProduct(Product{TenantID: "tenant-a", SKU: "P001", Name: "Disabled", Price: Money{Amount: "1.00", Currency: "EUR"}, TaxGroup: "B", Status: "INACTIVE"})
	if err != nil || product.Active {
		t.Fatalf("inactive product created as active: %#v %v", product, err)
	}
	employee, err := s.CreateEmployee(Employee{TenantID: "tenant-a", FirstName: "Former", LastName: "Cashier", OperatorCode: "I001", Roles: []string{"CASHIER"}, Status: "INACTIVE"})
	if err != nil || employee.Active {
		t.Fatalf("inactive employee created as active: %#v %v", employee, err)
	}
	if _, err = s.BindEmployeeIdentity("tenant-a", employee.ID, "former", "https://identity.example.test"); err == nil {
		t.Fatal("inactive-at-creation employee received identity authority")
	}
}

func TestIdentityIssuerCanonicalizationMatchesOIDCClaims(t *testing.T) {
	s := NewService("http://invalid", "2026-08-07")
	employee, err := s.CreateEmployee(Employee{TenantID: "tenant-a", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001", Roles: []string{"CASHIER"}, Status: "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := s.BindEmployeeIdentity("tenant-a", employee.ID, "cashier-1", " https://identity.example.test/ ")
	if err != nil || binding.IdentityIssuer != "https://identity.example.test" {
		t.Fatalf("issuer was not canonicalized: %#v %v", binding, err)
	}
	resolved, err := s.EmployeeForIdentity("tenant-a", "https://identity.example.test", "cashier-1")
	if err != nil || resolved.ID != employee.ID {
		t.Fatalf("canonical OIDC issuer did not resolve binding: %#v %v", resolved, err)
	}
}
