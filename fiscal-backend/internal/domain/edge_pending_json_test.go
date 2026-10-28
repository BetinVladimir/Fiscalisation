package domain

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPendingCommandJSONPreservesResultAndPayloadNumbers(t *testing.T) {
	raw := []byte(`{"operation_id":"operation","operation_sequence":9223372036854775807,"unp_sequence":0,"payload":{"nested":{"value":9007199254740993}},"result_payload":{"payments":[{"amount_minor":9223372036854775807}],"minimum":-9223372036854775808}}`)
	var pending EdgePendingCommand
	if err := json.Unmarshal(raw, &pending); err != nil {
		t.Fatal(err)
	}
	if pending.OperationSequence != 9223372036854775807 || intAny(pending.Payload["nested"].(map[string]any), "value") != 9007199254740993 {
		t.Fatal("typed sequence or downstream intAny lost precision")
	}
	encoded, err := json.Marshal(pending)
	if err != nil {
		t.Fatal(err)
	}
	for _, number := range []string{"9007199254740993", "9223372036854775807", "-9223372036854775808"} {
		if !bytes.Contains(encoded, []byte(number)) {
			t.Fatalf("lost %s: %s", number, encoded)
		}
	}
	if err = json.Unmarshal(append(raw, []byte(` {}`)...), &pending); err == nil {
		t.Fatal("multiple values accepted")
	}
}
