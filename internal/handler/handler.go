package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	models "github.com/RoGogDBD/metric-alerter/internal/model"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/go-chi/chi"
)

type Handler struct {
	storage repository.Storage
}

func NewHandler(storage repository.Storage) *Handler {
	return &Handler{storage: storage}
}

var (
	ErrUnknownMetricType = errors.New("unknown metric type")
	ErrInvalidValue      = errors.New("invalid value for metric type")
)

func ValidateMetricInput(metricType, metricName, metricValue string) (*repository.MetricUpdate, error) {
	switch metricType {
	case models.Gauge:
		v, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			return nil, err
		}
		return &repository.MetricUpdate{
			Type:     models.Gauge,
			Name:     metricName,
			FloatVal: &v,
		}, nil
	case models.Counter:
		v, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			return nil, err
		}
		return &repository.MetricUpdate{
			Type:   models.Counter,
			Name:   metricName,
			IntVal: &v,
		}, nil
	default:
		return nil, ErrUnknownMetricType
	}
}

func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")
	metricValue := chi.URLParam(r, "value")

	metric, err := ValidateMetricInput(metricType, metricName, metricValue)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrUnknownMetricType) {
			status = http.StatusNotImplemented
		}
		http.Error(w, err.Error(), status)
		return
	}
	switch metric.Type {
	case models.Gauge:
		h.storage.SetGauge(metric.Name, *metric.FloatVal)
	case models.Counter:
		h.storage.AddCounter(metric.Name, *metric.IntVal)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleUpdateJSON(w http.ResponseWriter, r *http.Request) {
	var m models.Metrics
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if m.ID == "" || m.MType == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	switch m.MType {
	case models.Gauge:
		if m.Value == nil {
			http.Error(w, "missing value for gauge", http.StatusBadRequest)
			return
		}
		h.storage.SetGauge(m.ID, *m.Value)

	case models.Counter:
		if m.Delta == nil {
			http.Error(w, "missing delta for counter", http.StatusBadRequest)
			return
		}
		h.storage.AddCounter(m.ID, *m.Delta)

	default:
		http.Error(w, "unknown metric type", http.StatusNotImplemented)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(m)
}

func (h *Handler) HandleGetMetricValue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")

	switch metricType {
	case models.Gauge:
		val, ok := h.storage.GetGauge(metricName)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Write([]byte(strconv.FormatFloat(val, 'f', -1, 64)))
	case models.Counter:
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

func (h *Handler) HandleGetMetricJSON(w http.ResponseWriter, r *http.Request) {
	var req models.Metrics
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if req.ID == "" || req.MType == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	switch req.MType {
	case models.Gauge:
		val, ok := h.storage.GetGauge(req.ID)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		req.Value = &val

	case models.Counter:
		val, ok := h.storage.GetCounter(req.ID)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		req.Delta = &val

	default:
		http.Error(w, "unknown metric type", http.StatusNotImplemented)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(req)
}

func (h *Handler) HandleMetricsPage(w http.ResponseWriter, r *http.Request) {
	metrics := h.storage.GetAll()

	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Name < metrics[j].Name
	})

	builder := strings.Builder{}
	builder.WriteString("<html><body><h1>Metrics</h1><ul>")
	for _, metric := range metrics {
		builder.WriteString("<li>" + metric.Name + ": " + metric.Value + "</li>")
	}
	builder.WriteString("</ul></body></html>")

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(builder.String()))
}
