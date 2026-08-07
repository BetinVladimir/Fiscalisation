package ble

import "testing"

func TestFlowReorderResumeDuplicateAndBusy(t *testing.T) {
	r := NewReassembler(10)
	var a, b [16]byte
	a[0] = 1
	b[0] = 2
	status, payload, err := r.Add(Frame{MessageID: a, ChunkIndex: 1, ChunkCount: 3}, []byte("B"))
	if err != nil || payload != nil || status.HighestContiguous != -1 || status.MissingBitmap&1 == 0 {
		t.Fatal(status, string(payload), err)
	}
	if _, _, err = r.Add(Frame{MessageID: b, ChunkIndex: 0, ChunkCount: 1}, []byte("other")); err == nil || err.Error() != "BUSY" {
		t.Fatal("second in-flight message accepted", err)
	}
	status = r.Resume(a)
	if status.HighestContiguous != -1 {
		t.Fatal(status)
	}
	_, _, err = r.Add(Frame{MessageID: a, ChunkIndex: 1, ChunkCount: 3}, []byte("B"))
	if err != nil {
		t.Fatal("identical duplicate rejected", err)
	}
	if _, _, err = r.Add(Frame{MessageID: a, ChunkIndex: 1, ChunkCount: 3}, []byte("changed")); err == nil {
		t.Fatal("conflicting duplicate accepted")
	}
	_, _, _ = r.Add(Frame{MessageID: a, ChunkIndex: 0, ChunkCount: 3}, []byte("A"))
	status, payload, err = r.Add(Frame{MessageID: a, ChunkIndex: 2, ChunkCount: 3}, []byte("C"))
	if err != nil || !status.Complete || string(payload) != "ABC" {
		t.Fatal(status, string(payload), err)
	}
}

func TestFlowCancelAndLimits(t *testing.T) {
	r := NewReassembler(2)
	var id [16]byte
	id[0] = 1
	if _, _, err := r.Add(Frame{MessageID: id, ChunkIndex: 0, ChunkCount: 3}, nil); err == nil {
		t.Fatal("oversized message accepted")
	}
	if _, _, err := r.Add(Frame{MessageID: id, ChunkIndex: 0, ChunkCount: 2}, []byte("a")); err != nil {
		t.Fatal(err)
	}
	if !r.Cancel(id) {
		t.Fatal("cancel failed")
	}
	if r.Resume(id).HighestContiguous != -1 {
		t.Fatal("cancelled message resumed")
	}
}
