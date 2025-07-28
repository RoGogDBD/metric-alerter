package repository

import (
	"strconv"
	"sync"
)

type Storage interface {
	SetGauge(name string, value float64)
	AddCounter(name string, delta int64)
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAll() []MetricInfo
}

type MemStorage struct {
	gauge   map[string]float64
	counter map[string]int64
	mu      sync.RWMutex
}

type MetricInfo struct {
	Name  string
	Type  string
	Value string
}

type MetricUpdate struct {
	Type     string
	Name     string
	FloatVal *float64
	IntVal   *int64
}

func NewMemStorage() Storage {
	return &MemStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int64),
	}
}

func (s *MemStorage) SetGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauge[name] = value
}

func (s *MemStorage) AddCounter(name string, delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter[name] += delta
}

func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.gauge[name]
	return val, ok
}

func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.counter[name]
	return val, ok
}

func (s *MemStorage) GetAll() []MetricInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MetricInfo
	for k, v := range s.gauge {
		result = append(result, MetricInfo{
			Name:  k,
			Type:  "gauge",
			Value: strconv.FormatFloat(v, 'f', -1, 64),
		})
	}
	for k, v := range s.counter {
		result = append(result, MetricInfo{
			Name:  k,
			Type:  "counter",
			Value: strconv.FormatInt(v, 10),
		})
	}
	return result
}
