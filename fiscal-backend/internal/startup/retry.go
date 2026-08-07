package startup

import (
	"context"
	"time"
)

func Retry[T any](ctx context.Context, interval time.Duration, open func() (T, error)) (T, error) {
	var zero T
	for {
		value, err := open()
		if err == nil {
			return value, nil
		}
		select {
		case <-ctx.Done():
			return zero, err
		case <-time.After(interval):
		}
	}
}
