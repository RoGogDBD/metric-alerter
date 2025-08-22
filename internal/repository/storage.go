package repository

import (
	"encoding/json"
	"os"
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

func (s *MemStorage) SaveToFile(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := make([]MetricUpdate, 0, len(s.gauge)+len(s.counter))

	for name, val := range s.gauge {
		data = append(data, MetricUpdate{
			Type:     "gauge",
			Name:     name,
			FloatVal: &val,
		})
	}
	for name, val := range s.counter {
		data = append(data, MetricUpdate{
			Type:   "counter",
			Name:   name,
			IntVal: &val,
		})
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, bytes, 0666)
}

// Загружает метрики из JSON-файла
func (s *MemStorage) LoadFromFile(path string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var data []MetricUpdate
	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range data {
		switch m.Type {
		case "gauge":
			if m.FloatVal != nil {
				s.gauge[m.Name] = *m.FloatVal
			}
		case "counter":
			if m.IntVal != nil {
				s.counter[m.Name] = *m.IntVal
			}
		}
	}
	return nil
}
