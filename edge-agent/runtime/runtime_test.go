package runtime

import (
	"encoding/json"
	"fiscalisation/edge-agent/authority"
	"fiscalisation/edge-agent/journal"
	"path/filepath"
	"testing"
	"time"
)

func setup(t *testing.T, d Device) (*Runtime, *journal.Journal) {
	t.Helper()
	j, e := journal.Open(filepath.Join(t.TempDir(), "edge.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	a := authority.New(authority.Lease{RegisterID: "r1", EdgeID: "e1", FencingToken: 7, OperationFrom: 1, OperationTo: 20, UNPFrom: 100, UNPTo: 120, ExpiresAt: time.Now().Add(time.Hour)})
	return New(j, a, d), j
}
func command() Command {
	return Command{CommandID: "cmd-1", TenantID: "tenant-1", RegisterID: "r1", DeviceID: "device-1", Type: "FISCAL_SALE", FencingToken: 7, Payload: json.RawMessage(`{"currency":"EUR"}`)}
}
func TestDurableBeforeExecuteAndIdempotent(t *testing.T) {
	d := &Simulator{Reachable: true}
	r, j := setup(t, d)
	v, e := r.Execute(command())
	if e != nil || v.State != "FISCALIZED" {
		t.Fatalf("%+v %v", v, e)
	}
	events := j.Events()
	if len(events) != 2 || events[0].Type != "COMMAND_DURABLE" || events[1].Type != "COMMAND_RESULT" {
		t.Fatalf("%+v", events)
	}
	v, e = r.Execute(command())
	if e != nil || d.Executions != 1 {
		t.Fatalf("duplicate executions=%d err=%v", d.Executions, e)
	}
}
func TestDeviceLossBlocksWithoutJournal(t *testing.T) {
	r, j := setup(t, &Simulator{Reachable: false})
	v, e := r.Execute(command())
	if e != nil || v.State != "BLOCKED" || len(j.Events()) != 0 {
		t.Fatalf("%+v %v", v, e)
	}
}
func TestTimeoutNeverBlindRetries(t *testing.T) {
	d := &Simulator{Reachable: true, FailAfterAccept: true}
	r, _ := setup(t, d)
	v, e := r.Execute(command())
	if e != nil || v.State != "FISCAL_RESULT_UNKNOWN" {
		t.Fatalf("%+v %v", v, e)
	}
	v, e = r.Execute(command())
	if e != nil || v.State != "FISCAL_RESULT_UNKNOWN" || d.Executions != 1 {
		t.Fatalf("%+v executions=%d", v, d.Executions)
	}
}

type knownFailureDevice struct{ Simulator }

func (d *knownFailureDevice) Execute(Command) (string, error) {
	d.Executions++
	return "", KnownDeviceFailure("PAPER_OUT")
}

func TestKnownDeviceFailureIsNotReportedUnknown(t *testing.T) {
	d := &knownFailureDevice{Simulator: Simulator{Reachable: true}}
	r, _ := setup(t, d)
	v, err := r.Execute(command())
	if err != nil || v.State != "FAILED" || v.ErrorCode != "PAPER_OUT" || d.Executions != 1 {
		t.Fatalf("unexpected known failure: %+v err=%v executions=%d", v, err, d.Executions)
	}
	v, err = r.Execute(command())
	if err != nil || v.State != "FAILED" || d.Executions != 1 {
		t.Fatal("known failure was executed again")
	}
}

func TestAllocationRestoredFromJournal(t *testing.T) {
	d := &Simulator{Reachable: true}
	r, j := setup(t, d)
	if _, e := r.Execute(command()); e != nil {
		t.Fatal(e)
	}
	a := authority.New(authority.Lease{RegisterID: "r1", EdgeID: "e1", FencingToken: 7, OperationFrom: 1, OperationTo: 20, UNPFrom: 100, UNPTo: 120, ExpiresAt: time.Now().Add(time.Hour)})
	r2 := New(j, a, d)
	c := command()
	c.CommandID = "cmd-2"
	v, e := r2.Execute(c)
	if e != nil || v.OperationSequence != 2 || v.UNPSequence != 101 {
		t.Fatalf("%+v %v", v, e)
	}
}
