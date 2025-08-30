package handler

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
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
	db      *sql.DB
}

func NewHandler(storage repository.Storage, db *sql.DB) *Handler {
	return &Handler{storage: storage, db: db}
}

var (
	ErrUnknownMetricType = errors.New("unknown metric type")
	ErrInvalidValue      = errors.New("invalid value for metric type")
)

func ValidateMetricInput(metricType, metricName, metricValue string) (*repository.MetricUpdate, error) {
	switch metricType {
	case "gauge":
		v, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			return nil, err
		}
		return &repository.MetricUpdate{
			Type:     "gauge",
			Name:     metricName,
			FloatVal: &v,
		}, nil
	case "counter":
		v, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			return nil, err
		}
		return &repository.MetricUpdate{
			Type:   "counter",
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
	case "gauge":
		h.storage.SetGauge(metric.Name, *metric.FloatVal)
	case "counter":
		h.storage.AddCounter(metric.Name, *metric.IntVal)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleGetMetricValue(w http.ResponseWriter, r *http.Request) {
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

func decodeRequestBody(r *http.Request, v interface{}) error {
	var reader io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			return err
		}
		defer gz.Close()
		reader = gz
	}
	return json.NewDecoder(reader).Decode(v)
}

func (h *Handler) HandleUpdateJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var m models.Metrics
	if err := decodeRequestBody(r, &m); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	switch m.MType {
	case "gauge":
		if m.Value == nil {
			http.Error(w, "missing value for gauge", http.StatusBadRequest)
			return
		}
		h.storage.SetGauge(m.ID, *m.Value)
	case "counter":
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

func (h *Handler) HandleGetMetricJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.Metrics
	if err := decodeRequestBody(r, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	resp := models.Metrics{
		ID:    req.ID,
		MType: req.MType,
	}
	switch req.MType {
	case "gauge":
		val, ok := h.storage.GetGauge(req.ID)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		resp.Value = &val
	case "counter":
		delta, ok := h.storage.GetCounter(req.ID)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		resp.Delta = &delta
	default:
		http.Error(w, "unknown metric type", http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) HandlePing(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}
	if err := h.db.Ping(); err != nil {
		http.Error(w, "database not reachable: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
