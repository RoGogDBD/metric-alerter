package agent

import "time"

// generate:reset
type MetricsBatch struct {
	Timestamp  time.Time
	MetricType string
	Values     []float64
	Labels     map[string]string
	Count      int
	IsActive   bool
}
