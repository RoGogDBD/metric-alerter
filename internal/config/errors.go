package config

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

var retryIntervals = []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

func RetryWithBackoff(ctx context.Context, op func() error) error {
	var lastErr error
	for i, wait := range retryIntervals {
		if err := op(); err != nil {
			if isRetriableError(err) {
				lastErr = err
				log.Printf("Retriable error: %v (attempt %d/%d). Retrying in %v...", err, i+1, len(retryIntervals), wait)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(wait):
					continue
				}
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("operation failed after retries: %w", lastErr)
}

func isRetriableError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if len(pgErr.Code) >= 2 && pgErr.Code[:2] == "08" {
			return true
		}
	}
	return false
}
