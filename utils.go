package main

import (
	"context"
	"time"
	"log"
	"fmt"
)

func retry(ctx context.Context, attempts int, base time.Duration, fn func() error) (err error,) {
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return
		}
		// dont sleep after last attempt
		if i == attempts-1 {
			break 
		}
		backoff := base * time.Duration(1<<i)
		log.Printf("attempt %d failed: %v; retrying in %s", i+1, err, backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
		return fmt.Errorf("after %d attempts: %w", attempts, err)
}