package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
)

func TestHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		url        string
		wantStatus int
	}{
		{
			name:       "Valid gauge update",
			method:     http.MethodPost,
			url:        "/update/gauge/Alloc/123.45",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Valid counter update",
			method:     http.MethodPost,
			url:        "/update/counter/PollCount/1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Invalid method",
			method:     http.MethodGet,
			url:        "/update/gauge/Alloc/123.45",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "Invalid type",
			method:     http.MethodPost,
			url:        "/update/unknown/Alloc/123.45",
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:       "Malformed path",
			method:     http.MethodPost,
			url:        "/invalidpath",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Bad value for counter",
			method:     http.MethodPost,
			url:        "/update/counter/PollCount/abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{
				storage: NewMemStorage(),
			}

			r := chi.NewRouter()
			r.Post("/update/{type}/{name}/{value}", handler.handleUpdate)
			r.Get("/value/{type}/{name}", handler.handleGetMetricValue)
			r.Get("/", handler.handleMetricsPage)

			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}
