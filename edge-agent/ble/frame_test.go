package ble

import "testing"

func TestCanonicalFrameRoundTripAndReplay(t *testing.T) {
	client, _ := NewEndpoint([]byte("pairing-secret"), "session-1", "client")
	edge, _ := NewEndpoint([]byte("pairing-secret"), "session-1", "edge")
	var id [16]byte
	copy(id[:], []byte("0123456789abcdef"))
	raw, err := client.SealFrame(id, 0, 1, 0x01, []byte("canonical-cbor"))
	if err != nil {
		t.Fatal(err)
	}
	frame, plain, err := edge.OpenFrame(raw)
	if err != nil || string(plain) != "canonical-cbor" || frame.Counter != 1 || frame.MessageID != id {
		t.Fatal(frame, string(plain), err)
	}
	if _, _, err = edge.OpenFrame(raw); err == nil {
		t.Fatal("frame replay accepted")
	}
	raw[len(raw)-1] ^= 1
	fresh, _ := NewEndpoint([]byte("pairing-secret"), "session-1", "edge")
	if _, _, err = fresh.OpenFrame(raw); err == nil {
		t.Fatal("corrupt GCM tag accepted")
	}
}

func TestMTUChunking(t *testing.T) {
	for _, mtu := range []int{185, 247, 517} {
		chunks, err := ChunkPlaintext(make([]byte, 1000), mtu)
		if err != nil {
			t.Fatal(mtu, err)
		}
		var total int
		for _, c := range chunks {
			total += len(c)
			if len(c) > mtu-3-HeaderSize-GCMTagSize {
				t.Fatal("oversized chunk")
			}
		}
		if total != 1000 {
			t.Fatal(mtu, total)
		}
	}
	if _, err := MaxChunkPlaintext(53); err == nil {
		t.Fatal("impossible MTU accepted")
	}
}
