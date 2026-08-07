package startup

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryRecoversTransientFailure(t *testing.T) {
	attempts := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := Retry(ctx, time.Millisecond, func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("dns not ready")
		}
		return "connected", nil
	})
	if err != nil || got != "connected" || attempts != 3 {
		t.Fatalf("got=%q attempts=%d err=%v", got, attempts, err)
	}
}

func TestRetryHonoursDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err := Retry(ctx, time.Millisecond, func() (string, error) { return "", errors.New("permanent") })
	if err == nil {
		t.Fatal("expected terminal startup error")
	}
}
