package runtime

import (
	"testing"
	"time"

	"fiscalisation/edge-agent/authority"
	"fiscalisation/edge-agent/journal"
)

func TestCriticalStorageBlocksBeforeProbeSequenceAndJournal(t *testing.T) {
	j, err := journal.Open(t.TempDir() + "/edge.db")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	d := &Simulator{Reachable: true}
	a := authority.New(authority.Lease{RegisterID: "r", EdgeID: "e", FencingToken: 1, OperationFrom: 1, OperationTo: 2, UNPFrom: 1, UNPTo: 2, ExpiresAt: time.Now().Add(time.Hour)})
	r := New(j, a, d)
	r.SetStorageQuota(1)
	result, err := r.Execute(Command{CommandID: "c", TenantID: "t", RegisterID: "r", DeviceID: "d", Type: "FISCAL_SALE", FencingToken: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "BLOCKED" || result.ErrorCode != "EDGE_STORAGE_CRITICAL" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if d.Executions != 0 || len(j.Events()) != 0 {
		t.Fatal("critical storage caused a device or journal side effect")
	}
}
