package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi"
)

type MemStorage struct {
	gauge   map[string]float64
	counter map[string]int64
	mu      sync.RWMutex
}

func NewMemStorage() *MemStorage {
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

func (s *MemStorage) GetAll() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range s.gauge {
		result[k+" (gauge)"] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	for k, v := range s.counter {
		result[k+" (counter)"] = strconv.FormatInt(v, 10)
	}
	return result
}

type Handler struct {
	storage *MemStorage
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")
	metricValue := chi.URLParam(r, "value")

	switch metricType {
	case "gauge":
		val, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			http.Error(w, "invalid gauge value", http.StatusBadRequest)
			return
		}
		h.storage.SetGauge(metricName, val)

	case "counter":
		val, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			http.Error(w, "invalid counter value", http.StatusBadRequest)
			return
		}
		h.storage.AddCounter(metricName, val)

	default:
		http.Error(w, "unknown metric type", http.StatusNotImplemented)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleGetMetricValue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")

	switch metricType {
	case "gauge":
		val, ok := h.storage.GetGauge(metricName)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Write([]byte(strconv.FormatFloat(val, 'f', -1, 64)))
	case "counter":
		val, ok := h.storage.GetCounter(metricName)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Write([]byte(strconv.FormatInt(val, 10)))
	default:
		http.Error(w, "invalid metric type", http.StatusBadRequest)
	}
}

func (h *Handler) handleMetricsPage(w http.ResponseWriter, r *http.Request) {
	metrics := h.storage.GetAll()

	builder := strings.Builder{}
	builder.WriteString("<html><body><h1>Metrics</h1><ul>")
	for name, value := range metrics {
		builder.WriteString("<li>" + name + ": " + value + "</li>")
	}
	builder.WriteString("</ul></body></html>")

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(builder.String()))
}

func main() {
	storage := NewMemStorage()
	handler := &Handler{storage: storage}

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", handler.handleUpdate)
	r.Get("/value/{type}/{name}", handler.handleGetMetricValue)
	r.Get("/", handler.handleMetricsPage)

	http.ListenAndServe(":8080", r)
}
