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
	"time"

	models "github.com/RoGogDBD/metric-alerter/internal/model"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler реализует обработчики HTTP-запросов для работы с метриками.
//
// Содержит хранилище метрик, подключение к базе данных, ключ для HMAC и менеджер аудита.
type Handler struct {
	storage      repository.Storage  // Хранилище метрик
	db           *pgxpool.Pool       // Подключение к базе данных
	key          string              // Ключ для HMAC-подписи
	auditManager models.AuditSubject // Менеджер аудита
}

// NewHandler создает новый экземпляр Handler.
//
// storage — реализация интерфейса Storage для хранения метрик.
// db — пул подключений к базе данных PostgreSQL.
func NewHandler(storage repository.Storage, db *pgxpool.Pool) *Handler {
	return &Handler{storage: storage, db: db}
}

// SetKey устанавливает ключ для HMAC-подписи ответов.
//
// key — секретный ключ для вычисления HMAC-SHA256 подписи.
// Если ключ пустой, подпись не вычисляется и не проверяется.
func (h *Handler) SetKey(key string) {
	h.key = key
}

// SetAuditManager устанавливает менеджер аудита для отправки событий.
//
// manager — менеджер аудита, реализующий интерфейс AuditSubject.
// После установки все операции с метриками будут отправлять события аудита через этот менеджер.
func (h *Handler) SetAuditManager(manager models.AuditSubject) {
	h.auditManager = manager
}

// getClientIP извлекает IP-адрес клиента из HTTP-запроса.
//
// Сначала проверяет заголовки X-Forwarded-For и X-Real-IP, затем RemoteAddr.
func (h *Handler) getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

// sendAuditEvent отправляет событие аудита с именами метрик и IP-адресом клиента.
//
// Если менеджер аудита не установлен, ничего не делает.
func (h *Handler) sendAuditEvent(r *http.Request, metricNames []string) {
	if h.auditManager == nil {
		return
	}

	event := models.AuditEvent{
		Timestamp: time.Now().Unix(),
		Metrics:   metricNames,
		IPAddress: h.getClientIP(r),
	}

	h.auditManager.Notify(event)
}

// computeHash вычисляет HMAC-SHA256 для переданных данных с использованием ключа Handler.
//
// Возвращает hex-представление подписи.
func (h *Handler) computeHash(data []byte) string {
	hash := hmac.New(sha256.New, []byte(h.key))
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// verifyHash проверяет корректность подписи HMAC-SHA256 для тела запроса.
//
// Если ключ или подпись не заданы, возвращает true.
func (h *Handler) verifyHash(body []byte, receivedHash string) bool {
	if h.key == "" {
		return true
	}
	if receivedHash == "" {
		return true
	}
	expectedHash := h.computeHash(body)
	return receivedHash == expectedHash
}

// writeJSONWithHash сериализует данные в JSON, добавляет подпись HMAC (если задан ключ) и пишет в ответ.
//
// Устанавливает Content-Type: application/json и HashSHA256 (если ключ задан).
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
	// ErrUnknownMetricType возвращается при попытке работы с неизвестным типом метрики.
	ErrUnknownMetricType = errors.New("unknown metric type")
)

// ValidateMetricInput валидирует входные параметры метрики и возвращает MetricUpdate.
//
// metricType — тип метрики ("gauge" или "counter").
// metricName — имя метрики.
// metricValue — значение метрики (строка).
//
// Возвращает MetricUpdate или ошибку.
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

// HandleUpdate обрабатывает PUT/POST-запросы для обновления значения метрики по URL.
//
// Ожидает параметры type, name, value в URL.
// Сохраняет метрику в хранилище и (если настроено) синхронизирует с БД.
// Отправляет событие аудита.
//
// @Summary Обновить метрику через URL
// @Description Обновляет значение метрики по параметрам в URL пути
// @Tags Metrics
// @Accept plain
// @Produce plain
// @Param type path string true "Тип метрики (gauge или counter)"
// @Param name path string true "Имя метрики"
// @Param value path string true "Значение метрики"
// @Success 200 {string} string "Метрика успешно обновлена"
// @Failure 400 {string} string "Некорректные параметры запроса"
// @Failure 501 {string} string "Неизвестный тип метрики"
// @Router /update/{type}/{name}/{value} [post]
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

	if h.db != nil {
		if err := repository.SyncToDB(r.Context(), h.storage, h.db); err != nil {
			log.Printf("Failed to sync metrics to DB: %v", err)
			http.Error(w, "failed to save metrics", http.StatusInternalServerError)
			return
		}
	}

	h.sendAuditEvent(r, []string{metricName})

	w.WriteHeader(http.StatusOK)
}

// HandleGetMetricValue возвращает значение метрики по имени и типу в виде текста.
//
// Ожидает параметры type и name в URL.
// Возвращает 404, если метрика не найдена.
//
// @Summary Получить значение метрики через URL
// @Description Возвращает значение метрики в виде текста
// @Tags Metrics
// @Produce plain
// @Param type path string true "Тип метрики (gauge или counter)"
// @Param name path string true "Имя метрики"
// @Success 200 {string} string "Значение метрики"
// @Failure 400 {string} string "Некорректный тип метрики"
// @Failure 404 {string} string "Метрика не найдена"
// @Router /value/{type}/{name} [get]
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

// HandleMetricsPage возвращает HTML-страницу со списком всех метрик.
//
// Формирует HTML-таблицу с именами и значениями метрик.
//
// @Summary Получить HTML-страницу со всеми метриками
// @Description Возвращает HTML-страницу со списком всех сохранённых метрик
// @Tags Metrics
// @Produce html
// @Success 200 {string} string "HTML-страница со списком метрик"
// @Router / [get]
func (h *Handler) HandleMetricsPage(w http.ResponseWriter, _ *http.Request) {
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

// decodeRequestBody декодирует тело запроса в структуру v.
//
// Поддерживает сжатие gzip, если установлен соответствующий заголовок.
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

// HandleUpdateJSON обрабатывает POST-запрос для обновления одной метрики в формате JSON.
//
// Проверяет подпись HMAC, валидирует и сохраняет метрику, синхронизирует с БД (если настроено), отправляет событие аудита.
//
// @Summary Обновить метрику в формате JSON
// @Description Обновляет значение одной метрики, переданной в теле запроса в формате JSON
// @Tags Metrics
// @Accept json
// @Produce json
// @Param metric body models.Metrics true "Метрика для обновления"
// @Param HashSHA256 header string false "HMAC-SHA256 подпись тела запроса"
// @Success 200 {object} models.Metrics "Обновлённая метрика"
// @Failure 400 {string} string "Некорректный JSON или неверная подпись"
// @Failure 500 {string} string "Ошибка сохранения метрики"
// @Router /update [post]
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
			http.Error(w, "failed to save metrics", http.StatusInternalServerError)
			return
		}
	}

	if err := h.writeJSONWithHash(w, m); err != nil {
		log.Printf("Failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

	h.sendAuditEvent(r, []string{m.ID})
}

// HandlerUpdateBatchJSON обрабатывает POST-запрос для пакетного обновления метрик в формате JSON.
//
// Проверяет подпись HMAC, валидирует и сохраняет каждую метрику, синхронизирует с БД (если настроено), отправляет событие аудита.
//
// @Summary Пакетное обновление метрик
// @Description Обновляет несколько метрик за один запрос, переданных в теле запроса в формате JSON
// @Tags Metrics
// @Accept json
// @Produce json
// @Param metrics body []models.Metrics true "Массив метрик для обновления"
// @Param HashSHA256 header string false "HMAC-SHA256 подпись тела запроса"
// @Success 200 {array} models.Metrics "Массив обновлённых метрик"
// @Failure 400 {string} string "Некорректный JSON или неверная подпись"
// @Failure 500 {string} string "Ошибка сохранения метрик"
// @Router /updates/ [post]
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
			http.Error(w, "failed to save metrics", http.StatusInternalServerError)
			return
		}
	}

	if err := h.writeJSONWithHash(w, metrics); err != nil {
		log.Printf("Failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

	metricNames := make([]string, len(metrics))
	for i, m := range metrics {
		metricNames[i] = m.ID
	}

	h.sendAuditEvent(r, metricNames)
}

// HandleGetMetricJSON обрабатывает POST-запрос для получения значения метрики в формате JSON.
//
// Ожидает структуру Metrics в теле запроса, возвращает значение метрики или ошибку.
//
// @Summary Получить значение метрики в формате JSON
// @Description Возвращает значение метрики по имени и типу, переданным в теле запроса
// @Tags Metrics
// @Accept json
// @Produce json
// @Param metric body models.Metrics true "Запрос метрики (id и type обязательны)"
// @Success 200 {object} models.Metrics "Метрика со значением"
// @Failure 400 {string} string "Некорректный JSON"
// @Failure 404 {string} string "Метрика не найдена"
// @Router /value [post]
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

// HandlePing проверяет доступность базы данных.
//
// Возвращает 200 OK, если соединение с БД успешно, иначе 500.
//
// @Summary Проверить доступность базы данных
// @Description Проверяет соединение с базой данных PostgreSQL
// @Tags Health
// @Produce plain
// @Success 200 {string} string "OK"
// @Failure 500 {string} string "База данных недоступна"
// @Router /ping [get]
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
