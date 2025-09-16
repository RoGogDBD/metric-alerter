package handler

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	models "github.com/RoGogDBD/metric-alerter/internal/model"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	storage repository.Storage
	db      *pgxpool.Pool
	key     string
}

func NewHandler(storage repository.Storage, db *pgxpool.Pool) *Handler {
	return &Handler{storage: storage, db: db}
}

func (h *Handler) SetKey(key string) {
	h.key = key
}

func (h *Handler) computeHash(data []byte) string {
	hash := hmac.New(sha256.New, []byte(h.key))
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

func (h *Handler) verifyHash(body []byte, receivedHash string) bool {
	if h.key == "" {
		return true
	}
	if receivedHash == "" {
		return false
	}
	expectedHash := h.computeHash(body)
	return receivedHash == expectedHash
}

func (h *Handler) writeJSONWithHash(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")

	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if h.key != "" {
		hash := h.computeHash(body)
		w.Header().Set("HashSHA256", hash)
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(body)
	return err
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

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	receivedHash := r.Header.Get("HashSHA256")
	if !h.verifyHash(body, receivedHash) {
		http.Error(w, "invalid signature", http.StatusBadRequest)
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

	if h.db != nil {
		if err := repository.SyncToDB(r.Context(), h.storage, h.db); err != nil {
			log.Printf("Failed to sync metrics to DB: %v", err)
		}
	}

	if err := h.writeJSONWithHash(w, m); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func (h *Handler) HandlerUpdateBatchJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	receivedHash := r.Header.Get("HashSHA256")
	if !h.verifyHash(body, receivedHash) {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	var metrics []models.Metrics
	if err := decodeRequestBody(r, &metrics); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	for _, m := range metrics {
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
	}

	if h.db != nil {
		if err := repository.SyncToDB(r.Context(), h.storage, h.db); err != nil {
			log.Printf("Failed to sync metrics to DB: %v", err)
		}
	}

	if err := h.writeJSONWithHash(w, metrics); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
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
	if err := h.writeJSONWithHash(w, resp); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func (h *Handler) HandlePing(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}
	if err := h.db.Ping(r.Context()); err != nil {
		http.Error(w, "database not reachable: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
