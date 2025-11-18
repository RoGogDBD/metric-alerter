package config

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestRetryWithBackoff тестирует функцию RetryWithBackoff на корректность обработки различных сценариев.
//
// Проверяются следующие случаи:
//   - Успешное выполнение после одной или нескольких повторных попыток (ретраев)
//   - Немедленный возврат ошибки, если ошибка не является временной (не ретраится)
//   - Исчерпание всех попыток с возвратом последней ошибки
//   - Прерывание по отмене контекста (context.Canceled)
//
// Для каждого теста задаются интервалы между попытками, фабрика операции (opFactory),
// ожидается ли ошибка, ожидается ли отмена контекста, минимальное количество вызовов,
// ожидаемый код ошибки PostgreSQL и ожидаемые сообщения об ошибках.
func TestRetryWithBackoff(t *testing.T) {
	delay := retryIntervals
	defer func() { retryIntervals = delay }()

	tests := []struct {
		name                  string                      // Название теста
		intervals             []time.Duration             // Интервалы между попытками
		opFactory             func() (func() error, *int) // Фабрика операции и счетчика вызовов
		expectErr             bool                        // Ожидается ли ошибка
		expectContextCanceled bool                        // Ожидается ли отмена контекста
		expectMinCalls        int                         // Минимальное количество вызовов операции
		expectPGCode          string                      // Ожидаемый код ошибки PostgreSQL
		expectMsgContains     string                      // Ожидаемая подстрока в сообщении об ошибке
		expectExactError      string                      // Ожидаемое точное сообщение об ошибке
	}{
		{
			name:      "SucceedsAfterRetry",
			intervals: []time.Duration{10 * time.Millisecond, 10 * time.Millisecond},
			opFactory: func() (func() error, *int) {
				calls := 0
				return func() error {
					calls++
					if calls == 1 {
						return &pgconn.PgError{Code: "08006", Message: "connection error"}
					}
					return nil
				}, &calls
			},
			expectErr:      false,
			expectMinCalls: 2,
		},
		{
			name:      "NonRetriableImmediate",
			intervals: []time.Duration{1 * time.Millisecond},
			opFactory: func() (func() error, *int) {
				calls := 0
				return func() error {
					calls++
					return errors.New("fatal")
				}, &calls
			},
			expectErr:        true,
			expectExactError: "fatal",
			expectMinCalls:   1,
		},
		{
			name:      "ExhaustRetries",
			intervals: []time.Duration{5 * time.Millisecond, 5 * time.Millisecond},
			opFactory: func() (func() error, *int) {
				calls := 0
				return func() error {
					calls++
					return &pgconn.PgError{Code: "08003", Message: "lost"}
				}, &calls
			},
			expectErr:         true,
			expectPGCode:      "08003",
			expectMsgContains: "operation failed after retries",
			expectMinCalls:    2,
		},
		{
			name:      "ContextCanceled",
			intervals: []time.Duration{200 * time.Millisecond},
			opFactory: func() (func() error, *int) {
				calls := 0
				return func() error {
					calls++
					return &pgconn.PgError{Code: "08006", Message: "connection error"}
				}, &calls
			},
			expectErr:             true,
			expectContextCanceled: true,
			expectMinCalls:        1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			retryIntervals = tt.intervals
			op, callsPtr := tt.opFactory()

			ctx := context.Background()
			var cancel context.CancelFunc
			if tt.expectContextCanceled {
				ctx, cancel = context.WithCancel(ctx)
				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()
				defer cancel()
			}

			err := RetryWithBackoff(ctx, op)

			if tt.expectContextCanceled {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("expected context.Canceled, got %v", err)
				}
			} else if !tt.expectErr {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.expectExactError != "" && err.Error() != tt.expectExactError {
					t.Fatalf("expected exact error %q, got %v", tt.expectExactError, err)
				}
				if tt.expectPGCode != "" {
					var pgErr *pgconn.PgError
					if !errors.As(err, &pgErr) || pgErr.Code != tt.expectPGCode {
						t.Fatalf("expected underlying pg error with code %s, got %v", tt.expectPGCode, err)
					}
				}
				if tt.expectMsgContains != "" && !strings.Contains(err.Error(), tt.expectMsgContains) {
					t.Fatalf("expected error message to contain %q, got %v", tt.expectMsgContains, err)
				}
			}

			if tt.expectMinCalls > 0 && (callsPtr == nil || *callsPtr < tt.expectMinCalls) {
				t.Fatalf("expected at least %d calls, got %d", tt.expectMinCalls, func() int {
					if callsPtr == nil {
						return 0
					}
					return *callsPtr
				}())
			}
		})
	}
}
