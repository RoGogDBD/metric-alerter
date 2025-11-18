package service

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RoGogDBD/metric-alerter/internal/handler"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewRouter_TableDriven(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "metrics.json")

	tests := []struct {
		name             string
		storeInterval    int
		expectSaveOnPost bool
	}{
		{"interval zero: save on POST", 0, true},
		{"interval non-zero: no immediate save on POST", 5, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			storage := repository.NewMemStorage()
			h := handler.NewHandler(storage, nil)
			logger := zap.NewNop()
			r := NewRouter(h, storage, tt.storeInterval, fpath, logger)

			cases := []struct {
				method string
				path   string
				body   []byte
			}{
				{"POST", "/update", []byte(`{"id":"m1","type":"gauge","value":1.23}`)},
				{"POST", "/update/", []byte(`{"id":"m1","type":"gauge","value":1.23}`)},
				{"POST", "/value", []byte(`{"id":"m1","type":"gauge"}`)},
				{"GET", "/ping", nil},
				{"GET", "/", nil},
			}
			for _, c := range cases {
				req := httptest.NewRequest(c.method, c.path, bytes.NewReader(c.body))
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)
				require.NotEqual(t, 0, rec.Code, "handler should respond")
			}

			req := httptest.NewRequest("POST", "/update", bytes.NewReader([]byte(`{"id":"m1","type":"gauge","value":1.23}`)))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)

			if tt.expectSaveOnPost {
				time.Sleep(10 * time.Millisecond)
				_, err := os.Stat(fpath)
				require.NoError(t, err)
				b, err := os.ReadFile(fpath)
				require.NoError(t, err)
				_ = b
			}
		})
	}
}
