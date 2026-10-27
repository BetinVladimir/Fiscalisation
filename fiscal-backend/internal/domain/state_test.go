package domain

import "testing"

func TestOperationStateTerminalSet(t *testing.T) {
	cases := []struct {
		state    OperationState
		terminal bool
	}{
		{StateExecuting, false},
		{StateFiscalized, true},
		{StateUnknown, true},
		{StateFailed, true},
		{StateRecovery, false},
	}
	for _, c := range cases {
		if got := c.state.Terminal(); got != c.terminal {
			t.Errorf("OperationState(%q).Terminal() = %v, want %v", c.state, got, c.terminal)
		}
	}
}
