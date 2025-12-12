package agent

import (
	"testing"
	"time"
)

func TestMetricsBatch_Reset(t *testing.T) {
	now := time.Now()
	batch := &MetricsBatch{
		Timestamp:  now,
		MetricType: "gauge",
		Values:     []float64{1.5, 2.3, 3.7, 4.1},
		Labels:     map[string]string{"host": "server1", "env": "prod"},
		Count:      42,
		IsActive:   true,
	}

	if batch.Count != 42 {
		t.Errorf("Expected Count=42, got %d", batch.Count)
	}
	if !batch.IsActive {
		t.Error("Expected IsActive=true")
	}
	if len(batch.Values) != 4 {
		t.Errorf("Expected Values len=4, got %d", len(batch.Values))
	}
	if len(batch.Labels) != 2 {
		t.Errorf("Expected Labels len=2, got %d", len(batch.Labels))
	}

	batch.Reset()

	if !batch.Timestamp.IsZero() {
		t.Errorf("Expected Timestamp to be zero after reset, got %v", batch.Timestamp)
	}
	if batch.MetricType != "" {
		t.Errorf("Expected MetricType='' after reset, got %s", batch.MetricType)
	}
	if len(batch.Values) != 0 {
		t.Errorf("Expected Values len=0 after reset, got %d", len(batch.Values))
	}
	if cap(batch.Values) != 4 {
		t.Errorf("Expected Values cap=4 after reset (slice should be truncated, not nil), got %d", cap(batch.Values))
	}
	if len(batch.Labels) != 0 {
		t.Errorf("Expected Labels len=0 after reset, got %d", len(batch.Labels))
	}
	if batch.Count != 0 {
		t.Errorf("Expected Count=0 after reset, got %d", batch.Count)
	}
	if batch.IsActive {
		t.Error("Expected IsActive=false after reset")
	}
}

func TestMetricsBatch_ResetNilPointer(t *testing.T) {
	var batch *MetricsBatch

	batch.Reset()
}
