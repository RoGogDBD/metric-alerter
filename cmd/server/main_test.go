package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RoGogDBD/metric-alerter/internal/handler"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/go-chi/chi/v5"
)

// TestHandler_ServeHTTP тестирует обработку различных HTTP-запросов к серверу метрик.
//
// Проверяет корректность обработки следующих сценариев:
//   - Обновление метрики типа gauge (POST /update/gauge/Alloc/123.45)
//   - Обновление метрики типа counter (POST /update/counter/PollCount/1)
//   - Некорректный HTTP-метод (GET вместо POST)
//   - Неизвестный тип метрики (POST /update/unknown/Alloc/123.45)
//   - Некорректный путь (POST /invalidpath)
//   - Некорректное значение для counter (POST /update/counter/PollCount/abc)
//
// Для каждого случая проверяется ожидаемый HTTP-статус ответа.
//
// t — указатель на структуру тестирования *testing.T.
func TestHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name       string // Название теста
		method     string // HTTP-метод запроса
		url        string // URL запроса
		wantStatus int    // Ожидаемый HTTP-статус ответа
	}{
		{
			name:       "Valid gauge update",
			method:     http.MethodPost,
			url:        "/update/gauge/Alloc/123.45",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Valid counter update",
			method:     http.MethodPost,
			url:        "/update/counter/PollCount/1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Invalid method",
			method:     http.MethodGet,
			url:        "/update/gauge/Alloc/123.45",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "Invalid type",
			method:     http.MethodPost,
			url:        "/update/unknown/Alloc/123.45",
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:       "Malformed path",
			method:     http.MethodPost,
			url:        "/invalidpath",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Bad value for counter",
			method:     http.MethodPost,
			url:        "/update/counter/PollCount/abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Инициализация in-memory хранилища и обработчика
			storage := repository.NewMemStorage()
			handler := handler.NewHandler(storage, nil)

			// Настройка маршрутов chi
			r := chi.NewRouter()
			r.Post("/update/{type}/{name}/{value}", handler.HandleUpdate)
			r.Get("/value/{type}/{name}", handler.HandleGetMetricValue)
			r.Get("/", handler.HandleMetricsPage)

			// Создание HTTP-запроса и запись ответа
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			// Проверка соответствия статуса ответа ожидаемому
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}
