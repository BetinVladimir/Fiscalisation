package startup

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetry(t *testing.T) {
	attempts := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := Retry(ctx, time.Millisecond, func() (int, error) {
		attempts++
		if attempts == 1 {
			return 0, errors.New("transient")
		}
		return 42, nil
	})
	if err != nil || got != 42 || attempts != 2 {
		t.Fatalf("got=%d attempts=%d err=%v", got, attempts, err)
	}
}
