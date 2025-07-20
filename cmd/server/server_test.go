package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
			wantStatus: http.StatusBadRequest,
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
				gauge:   make(map[string]float64),
				counter: make(map[string]int64),
			}

			req := httptest.NewRequest(tt.method, tt.url, strings.NewReader(""))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}
