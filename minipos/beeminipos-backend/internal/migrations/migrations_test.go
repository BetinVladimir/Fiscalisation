package migrations

import (
	"context"
	"testing"
)

func TestSchemaGateAllowsMemoryMode(t *testing.T) {
	if err := Run(context.Background(), ""); err != nil {
		t.Fatalf("memory mode schema gate: %v", err)
	}
}
