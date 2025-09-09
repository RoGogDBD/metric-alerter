package main

import (
	"compress/gzip"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	models "github.com/RoGogDBD/metric-alerter/internal/model"
	"github.com/go-resty/resty/v2"
)

func floatPtr(f float64) *float64 { return &f }
func int64Ptr(i float64) *int64 {
	v := int64(i)
	return &v
}

func TestSendMetrics(t *testing.T) {
	tests := []struct {
		name     string
		metric   Metric
		expected models.Metrics
		status   int
	}{
		{
			name:   "GaugeSuccess",
			metric: Metric{"gauge", 12.3},
			expected: models.Metrics{
				ID:    "TestMetric",
				MType: "gauge",
				Value: floatPtr(12.3),
				Delta: nil,
			},
			status: http.StatusOK,
		},
		{
			name:   "CounterSuccess",
			metric: Metric{"counter", 5},
			expected: models.Metrics{
				ID:    "TestMetric",
				MType: "counter",
				Value: nil,
				Delta: int64Ptr(5),
			},
			status: http.StatusOK,
		},
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

			var got []models.Metrics

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()

				if r.URL.Path != "/updates/" {
					t.Errorf("expected path /updates/, got %q", r.URL.Path)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
				}

				var reader = r.Body
				if r.Header.Get("Content-Encoding") == "gzip" {
					gz, err := gzip.NewReader(r.Body)
					if err != nil {
						t.Errorf("failed to create gzip reader: %v", err)
						return
					}
					defer gz.Close()
					reader = gz
				}
				if err := json.NewDecoder(reader).Decode(&got); err != nil {
					t.Errorf("failed to decode body: %v", err)
				}
				w.WriteHeader(tc.status)
			}))
			defer ts.Close()

			client := resty.New().SetBaseURL(ts.URL)
			state.Sender = &RestySender{Client: client}

			sendMetrics(state)

			if len(got) == 0 {
				t.Fatalf("no metrics were sent")
			}

			metricSent := got[0]

			if metricSent.ID != tc.expected.ID {
				t.Errorf("expected ID %q, got %q", tc.expected.ID, metricSent.ID)
			}
			if metricSent.MType != tc.expected.MType {
				t.Errorf("expected MType %q, got %q", tc.expected.MType, metricSent.MType)
			}
			if tc.expected.Value != nil && (metricSent.Value == nil || *metricSent.Value != *tc.expected.Value) {
				t.Errorf("expected Value %v, got %v", *tc.expected.Value, metricSent.Value)
			}
			if tc.expected.Delta != nil && (metricSent.Delta == nil || *metricSent.Delta != *tc.expected.Delta) {
				t.Errorf("expected Delta %v, got %v", *tc.expected.Delta, metricSent.Delta)
			}
		})
	}
}
