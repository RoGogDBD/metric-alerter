package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
)

func TestSendMetrics(t *testing.T) {
	tests := []struct {
		name     string
		metric   interface{}
		expected string
		status   int
	}{
		{"GaugeSuccess", 12.3, "/update/gauge/TestMetric/12.3", http.StatusOK},
		{"CounterSuccess", int64(5), "/update/counter/TestMetric/5", http.StatusOK},
		{"UnknownType", "unsupported", "", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetMetrics()
			metrics["TestMetric"] = tc.metric

			var actualPath string

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				actualPath = r.URL.Path
				io.ReadAll(r.Body)
				w.WriteHeader(tc.status)
			}))
			defer ts.Close()

			client := resty.New().SetBaseURL(ts.URL)
			sendMetrics(client)

			if tc.expected != "" && actualPath != tc.expected {
				t.Errorf("expected path %q, got %q", tc.expected, actualPath)
			}
		})
	}
}

func resetMetrics() {
	for k := range metrics {
		delete(metrics, k)
	}
	pollCount = 0
}
