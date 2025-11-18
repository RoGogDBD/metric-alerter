package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestInitialize_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		level string
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
			// flush
			_ = logger.Sync()
		})
	}
}

func TestRequestLogger_TableDriven(t *testing.T) {
	logger := zap.NewNop()
	middleware := RequestLogger(logger)

	tests := []struct {
		name       string
		status     int
		body       string
		expStatus  int
		expBodyLen int
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
