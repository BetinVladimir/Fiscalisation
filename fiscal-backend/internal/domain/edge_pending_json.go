package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// Preserve the authenticated canonical payload at every persistence/read path,
// not only at transport ingress. Other resource/nomenclature decoding remains
// unchanged; typed sequence and acceptance fields retain their existing types.
func (p *EdgePendingCommand) UnmarshalJSON(data []byte) error {
	type wire EdgePendingCommand
	var value wire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("pending command must contain exactly one JSON value")
	}
	*p = EdgePendingCommand(value)
	return nil
}
