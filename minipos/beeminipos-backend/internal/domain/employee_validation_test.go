package domain

import "testing"

func TestValidEmployeeAllowsEmptyLastName(t *testing.T) {
	if !validEmployee(Employee{FirstName: "Влад", LastName: "", OperatorCode: "0001", Roles: []string{"ADMIN"}, Status: "ACTIVE"}) {
		t.Fatal("employee with a single name was rejected")
	}
}
