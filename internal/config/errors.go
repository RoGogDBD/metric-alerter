package config

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// retryIntervals определяет интервалы ожидания между попытками повторения операции.
var retryIntervals = []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

// RetryWithBackoff выполняет функцию op с повторными попытками и экспоненциальной задержкой между ними.
//
// Если функция op возвращает ошибку, которая считается временной (retriable),
// происходит повторная попытка выполнения с увеличивающимся интервалом ожидания.
// Если все попытки исчерпаны или контекст завершён, возвращается последняя ошибка.
//
// ctx — контекст для управления временем жизни попыток.
// op  — функция, которую требуется выполнить с повторными попытками.
//
// Возвращает nil при успехе или ошибку, если операция не удалась после всех попыток.
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

// isRetriableError определяет, является ли ошибка временной (retriable) для PostgreSQL.
//
// err — ошибка для проверки.
//
// Возвращает true, если ошибка связана с проблемами соединения (коды SQLSTATE, начинающиеся с "08").
func isRetriableError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if len(pgErr.Code) >= 2 && pgErr.Code[:2] == "08" {
			return true
		}
	}
	return false
}
