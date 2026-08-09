package domain

import (
	"encoding/json"
	"testing"
)

func TestOrderAllowedActionsAreDerivedFromAuthoritativeState(t *testing.T) {
	cases := []struct {
		order Order
		want  []string
	}{
		{Order{State: "DRAFT"}, []string{"ADD_LINE"}},
		{Order{State: "DRAFT", Lines: []Line{{LineID: "line"}}}, []string{"ADD_LINE", "CHECKOUT"}},
		{Order{State: "UNKNOWN"}, []string{"READ"}},
		{Order{State: "FISCAL_PENDING"}, []string{"READ"}},
		{Order{State: "COMPLETED"}, []string{"RECEIPT", "REVERSE"}},
		{Order{State: "REVERSED"}, []string{"RECEIPT"}},
		{Order{State: "FAILED"}, []string{}},
	}
	for _, tc := range cases {
		raw, err := json.Marshal(tc.order)
		if err != nil {
			t.Fatal(err)
		}
		var response struct {
			AllowedActions []string `json:"allowed_actions"`
		}
		if err = json.Unmarshal(raw, &response); err != nil {
			t.Fatal(err)
		}
		if len(response.AllowedActions) != len(tc.want) {
			t.Fatalf("state %s actions=%v want=%v", tc.order.State, response.AllowedActions, tc.want)
		}
		for index := range tc.want {
			if response.AllowedActions[index] != tc.want[index] {
				t.Fatalf("state %s actions=%v want=%v", tc.order.State, response.AllowedActions, tc.want)
			}
		}
	}
}
