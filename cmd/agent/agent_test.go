package main

import (
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
)

func TestSendMetrics(t *testing.T) {
	tests := []struct {
		name     string
		metric   Metric
		expected string
		status   int
	}{
		{"GaugeSuccess", Metric{"gauge", 12.3}, "/update/gauge/TestMetric/12.3", http.StatusOK},
		{"CounterSuccess", Metric{"counter", 5}, "/update/counter/TestMetric/5", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := &AgentState{
				PollInterval:   2,
				ReportInterval: 10,
				PollCount:      0,
				Metrics:        map[string]Metric{"TestMetric": tc.metric},
				Rng:            rand.New(rand.NewSource(1)),
			}

			var actualPath string

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				actualPath = r.URL.Path
				io.ReadAll(r.Body)
				w.WriteHeader(tc.status)
			}))
			defer ts.Close()

			client := resty.New().SetBaseURL(ts.URL)
			state.Sender = &RestySender{Client: client}

			sendMetrics(state)

			if tc.expected != "" && actualPath != tc.expected {
				t.Errorf("expected path %q, got %q", tc.expected, actualPath)
			}
		})
	}
}
