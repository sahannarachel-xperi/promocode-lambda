package utils

import (
	"context"
	"time"
)

func RetryWithBackoff(ctx context.Context, operation func() error) error {
	backoff := 100 * time.Millisecond
	maxRetries := 3

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := operation(); err != nil {
			lastErr = err
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		return nil
	}
	return lastErr
}
