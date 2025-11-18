package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"go.uber.org/zap"
)

// TestInitialize_TableDriven выполняет табличные тесты для функции Initialize.
//
// Проверяет, что инициализация логгера проходит без ошибок для различных уровней логирования.
// После выполнения тестов удаляет директорию ./logs.
func TestInitialize_TableDriven(t *testing.T) {
	tests := []struct {
		name  string // Название теста
		level string // Уровень логирования для инициализации
	}{
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"error", "error"},
		{"invalid", "notalevel"},
	}

	defer func() {
		_ = os.RemoveAll("./logs")
	}()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			logger, err := Initialize(tt.level)
			if err != nil {
				t.Fatalf("Initialize(%q) returned error: %v", tt.level, err)
			}
			if logger == nil {
				t.Fatalf("Initialize(%q) returned nil logger", tt.level)
			}
			// Завершает работу логгера, сбрасывая буферы.
			_ = logger.Sync()
		})
	}
}

// TestRequestLogger_TableDriven выполняет табличные тесты для middleware RequestLogger.
//
// Проверяет, что middleware корректно обрабатывает различные HTTP-статусы и длины тела ответа.
// Для каждого теста создаётся обработчик, который возвращает заданный статус и тело ответа.
// Проверяется, что статус и длина тела ответа соответствуют ожидаемым значениям.
func TestRequestLogger_TableDriven(t *testing.T) {
	logger := zap.NewNop()
	middleware := RequestLogger(logger)

	tests := []struct {
		name       string // Название теста
		status     int    // HTTP-статус, который возвращает обработчик
		body       string // Тело ответа
		expStatus  int    // Ожидаемый HTTP-статус
		expBodyLen int    // Ожидаемая длина тела ответа
	}{
		{"ok small", http.StatusOK, "ok", http.StatusOK, len("ok")},
		{"created", http.StatusCreated, "created-response", http.StatusCreated, len("created-response")},
		{"no body", http.StatusNoContent, "", http.StatusNoContent, 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				if len(tt.body) > 0 {
					_, _ = w.Write([]byte(tt.body))
				}
			})

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)
			wrapped := middleware(h)
			wrapped.ServeHTTP(rec, req)

			if rec.Code != tt.expStatus {
				t.Fatalf("expected status %d, got %d", tt.expStatus, rec.Code)
			}
			if rec.Body.Len() != tt.expBodyLen {
				t.Fatalf("expected body len %d, got %d", tt.expBodyLen, rec.Body.Len())
			}
		})
	}
}
