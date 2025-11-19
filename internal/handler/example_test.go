package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/RoGogDBD/metric-alerter/internal/handler"
	models "github.com/RoGogDBD/metric-alerter/internal/model"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/go-chi/chi"
)

// ExampleHandler_HandleUpdate демонстрирует использование эндпоинта обновления метрики через URL.
//
// Показывает, как отправить POST-запрос на /update/{type}/{name}/{value}
// для обновления значения метрики.
func ExampleHandler_HandleUpdate() {
	// Создаём хранилище метрик
	storage := repository.NewMemStorage()
	h := handler.NewHandler(storage, nil)

	// Создаём запрос с параметрами URL для chi router
	req := httptest.NewRequest("POST", "/update/gauge/cpu_usage/75.5", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("type", "gauge")
	rctx.URLParams.Add("name", "cpu_usage")
	rctx.URLParams.Add("value", "75.5")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.HandleUpdate(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
	// Output:
	// Status: 200 OK
}

// ExampleHandler_HandleGetMetricValue демонстрирует использование эндпоинта получения значения метрики через URL.
//
// Показывает, как отправить GET-запрос на /value/{type}/{name}
// для получения значения метрики.
func ExampleHandler_HandleGetMetricValue() {
	// Создаём хранилище метрик и добавляем метрику
	storage := repository.NewMemStorage()
	storage.SetGauge("cpu_usage", 75.5)
	h := handler.NewHandler(storage, nil)

	// Создаём запрос с параметрами URL для chi router
	req := httptest.NewRequest("GET", "/value/gauge/cpu_usage", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("type", "gauge")
	rctx.URLParams.Add("name", "cpu_usage")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.HandleGetMetricValue(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %s, Value: %s\n", resp.Status, string(body))
	// Output:
	// Status: 200 OK, Value: 75.5
}

// ExampleHandler_HandleUpdateJSON демонстрирует использование эндпоинта обновления метрики в формате JSON.
//
// Показывает, как отправить POST-запрос на /update
// с телом запроса в формате JSON для обновления значения метрики.
func ExampleHandler_HandleUpdateJSON() {
	// Создаём хранилище метрик
	storage := repository.NewMemStorage()
	h := handler.NewHandler(storage, nil)

	// Создаём тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(h.HandleUpdateJSON))
	defer server.Close()

	// Подготавливаем метрику для отправки
	metric := models.Metrics{
		ID:    "cpu_usage",
		MType: "gauge",
	}
	value := 85.3
	metric.Value = &value

	// Сериализуем в JSON
	jsonData, _ := json.Marshal(metric)

	// Отправляем POST-запрос
	resp, err := http.Post(server.URL+"/update", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Читаем ответ
	var updatedMetric models.Metrics
	json.NewDecoder(resp.Body).Decode(&updatedMetric)

	fmt.Printf("Status: %s, Metric ID: %s, Value: %.1f\n", resp.Status, updatedMetric.ID, *updatedMetric.Value)
	// Output:
	// Status: 200 OK, Metric ID: cpu_usage, Value: 85.3
}

// ExampleHandler_HandleGetMetricJSON демонстрирует использование эндпоинта получения значения метрики в формате JSON.
//
// Показывает, как отправить POST-запрос на /value
// с телом запроса в формате JSON для получения значения метрики.
func ExampleHandler_HandleGetMetricJSON() {
	// Создаём хранилище метрик и добавляем метрику
	storage := repository.NewMemStorage()
	storage.SetGauge("memory_usage", 62.7)
	h := handler.NewHandler(storage, nil)

	// Создаём тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(h.HandleGetMetricJSON))
	defer server.Close()

	// Подготавливаем запрос
	req := models.Metrics{
		ID:    "memory_usage",
		MType: "gauge",
	}
	jsonData, _ := json.Marshal(req)

	// Отправляем POST-запрос
	resp, err := http.Post(server.URL+"/value", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Читаем ответ
	var metric models.Metrics
	json.NewDecoder(resp.Body).Decode(&metric)

	fmt.Printf("Status: %s, ID: %s, Value: %.1f\n", resp.Status, metric.ID, *metric.Value)
	// Output:
	// Status: 200 OK, ID: memory_usage, Value: 62.7
}

// ExampleHandler_HandlerUpdateBatchJSON демонстрирует использование эндпоинта пакетного обновления метрик.
//
// Показывает, как отправить POST-запрос на /updates/
// с массивом метрик в формате JSON для обновления нескольких метрик за один запрос.
func ExampleHandler_HandlerUpdateBatchJSON() {
	// Создаём хранилище метрик
	storage := repository.NewMemStorage()
	h := handler.NewHandler(storage, nil)

	// Создаём тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(h.HandlerUpdateBatchJSON))
	defer server.Close()

	// Подготавливаем массив метрик
	gaugeValue := 45.8
	counterDelta := int64(10)
	metrics := []models.Metrics{
		{
			ID:    "cpu_usage",
			MType: "gauge",
			Value: &gaugeValue,
		},
		{
			ID:    "request_count",
			MType: "counter",
			Delta: &counterDelta,
		},
	}

	// Сериализуем в JSON
	jsonData, _ := json.Marshal(metrics)

	// Отправляем POST-запрос
	resp, err := http.Post(server.URL+"/updates/", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Читаем ответ
	var updatedMetrics []models.Metrics
	json.NewDecoder(resp.Body).Decode(&updatedMetrics)

	fmt.Printf("Status: %s, Updated metrics: %d\n", resp.Status, len(updatedMetrics))
	// Output:
	// Status: 200 OK, Updated metrics: 2
}

// ExampleHandler_HandleMetricsPage демонстрирует использование эндпоинта получения HTML-страницы со всеми метриками.
//
// Показывает, как отправить GET-запрос на /
// для получения HTML-страницы со списком всех метрик.
func ExampleHandler_HandleMetricsPage() {
	// Создаём хранилище метрик и добавляем метрики
	storage := repository.NewMemStorage()
	storage.SetGauge("cpu_usage", 75.5)
	storage.AddCounter("request_count", 100)
	h := handler.NewHandler(storage, nil)

	// Создаём тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(h.HandleMetricsPage))
	defer server.Close()

	// Отправляем GET-запрос
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Проверяем наличие метрик в ответе
	hasCPU := strings.Contains(bodyStr, "cpu_usage")
	hasRequest := strings.Contains(bodyStr, "request_count")

	fmt.Printf("Status: %s, Contains cpu_usage: %v, Contains request_count: %v\n", resp.Status, hasCPU, hasRequest)
	// Output:
	// Status: 200 OK, Contains cpu_usage: true, Contains request_count: true
}

// ExampleHandler_HandlePing демонстрирует использование эндпоинта проверки доступности базы данных.
//
// Показывает, как отправить GET-запрос на /ping
// для проверки соединения с базой данных.
func ExampleHandler_HandlePing() {
	// Создаём хранилище метрик без подключения к БД
	storage := repository.NewMemStorage()
	h := handler.NewHandler(storage, nil)

	// Создаём тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(h.HandlePing))
	defer server.Close()

	// Отправляем GET-запрос
	resp, err := http.Get(server.URL + "/ping")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %s, Body: %s\n", resp.Status, string(body))
	// Output:
	// Status: 500 Internal Server Error, Body: database not configured
}

// ExampleHandler_Counter демонстрирует работу со счётчиками (counter).
//
// Показывает, как обновлять и получать значения счётчиков через JSON API.
func ExampleHandler_counter() {
	storage := repository.NewMemStorage()
	h := handler.NewHandler(storage, nil)
	server := httptest.NewServer(http.HandlerFunc(h.HandleUpdateJSON))
	defer server.Close()

	// Увеличиваем счётчик на 5
	delta1 := int64(5)
	metric1 := models.Metrics{
		ID:    "request_count",
		MType: "counter",
		Delta: &delta1,
	}
	jsonData1, _ := json.Marshal(metric1)
	resp1, _ := http.Post(server.URL+"/update", "application/json", bytes.NewReader(jsonData1))
	resp1.Body.Close()

	// Увеличиваем счётчик ещё на 3
	delta2 := int64(3)
	metric2 := models.Metrics{
		ID:    "request_count",
		MType: "counter",
		Delta: &delta2,
	}
	jsonData2, _ := json.Marshal(metric2)
	resp2, _ := http.Post(server.URL+"/update", "application/json", bytes.NewReader(jsonData2))
	resp2.Body.Close()

	// Получаем итоговое значение
	req := models.Metrics{ID: "request_count", MType: "counter"}
	reqData, _ := json.Marshal(req)

	getServer := httptest.NewServer(http.HandlerFunc(h.HandleGetMetricJSON))
	defer getServer.Close()
	resp3, _ := http.Post(getServer.URL+"/value", "application/json", bytes.NewReader(reqData))
	var result models.Metrics
	json.NewDecoder(resp3.Body).Decode(&result)
	resp3.Body.Close()

	fmt.Printf("Counter value after increments: %d\n", *result.Delta)
	// Output:
	// Counter value after increments: 8
}
