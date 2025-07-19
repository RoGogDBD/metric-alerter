package main

import (
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	gauge   map[string]float64
	counter map[string]int64
}

// Проверка соответствия интерфейса (Uber-Go/guide)
// Если интерфейс не реализован, то будет ошибка компиляции
var _ http.Handler = (*Handler)(nil)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain")

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) != 5 || parts[1] != "update" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	metricType := parts[2]
	metricName := parts[3]
	metricValue := parts[4]

	if metricName == "" {
		http.Error(w, "metric name required", http.StatusNotFound)
		return
	}

	switch metricType {
	case "gauge":
		val, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			http.Error(w, "Invalid gauge value", http.StatusBadRequest)
			return
		}
		h.gauge[metricName] = val
		w.WriteHeader(http.StatusOK)
		return

	case "counter":
		val, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			http.Error(w, "Invalid gauge value", http.StatusBadRequest)
			return
		}
		h.counter[metricName] += val
		w.WriteHeader(http.StatusOK)
		return

	default:
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
}

func main() {
	h := Handler{
		gauge:   make(map[string]float64),
		counter: make(map[string]int64),
	}
	if err := run(h); err != nil {
		panic(err)
	}
}

func run(h Handler) error {
	return http.ListenAndServe(`:8080`, &h)
}
