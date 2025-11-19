package service

import (
	"log"
	"net/http"
	"time"

	"github.com/RoGogDBD/metric-alerter/internal/config"
	"github.com/RoGogDBD/metric-alerter/internal/handler"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"go.uber.org/zap"
)

// NewRouter создает и настраивает HTTP-роутер для сервиса метрик.
// В зависимости от значения storeInterval, роутер либо сохраняет метрики в файл после каждого обновления,
// либо запускает отдельную горутину для периодического сохранения метрик.
//
// Параметры:
//   - h: обработчик запросов (handler.Handler)
//   - storage: хранилище метрик (repository.Storage)
//   - storeInterval: интервал сохранения метрик в файл (в секундах); если 0 — сохраняет после каждого обновления
//   - filePath: путь к файлу для сохранения метрик
//   - logger: логгер для логирования запросов
//
// Возвращает:
//   - *chi.Mux: настроенный роутер
func NewRouter(h *handler.Handler, storage repository.Storage, storeInterval int, filePath string, logger *zap.Logger) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)         // Добавляет уникальный идентификатор запроса
	r.Use(middleware.RealIP)            // Определяет реальный IP клиента
	r.Use(config.RequestLogger(logger)) // Логирует запросы с помощью zap
	r.Use(middleware.Recoverer)         // Восстанавливает после паники
	r.Use(middleware.Compress(5))       // Сжимает ответы

	if storeInterval == 0 {
		// Если storeInterval == 0, сохраняет метрики в файл после каждого обновления
		r.Post("/update", func(w http.ResponseWriter, r *http.Request) {
			h.HandleUpdateJSON(w, r)
			if err := repository.SaveMetricsToFile(storage, filePath); err != nil {
				log.Printf("Failed to save metrics: %v", err)
			}
		})
		r.Post("/update/", func(w http.ResponseWriter, r *http.Request) {
			h.HandleUpdateJSON(w, r)
			if err := repository.SaveMetricsToFile(storage, filePath); err != nil {
				log.Printf("Failed to save metrics: %v", err)
			}
		})
	} else {
		// Если storeInterval > 0, запускает периодическое сохранение метрик в отдельной горутине
		go func() {
			ticker := time.NewTicker(time.Duration(storeInterval) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if err := repository.SaveMetricsToFile(storage, filePath); err != nil {
					log.Printf("Failed to save metrics: %v", err)
				}
			}
		}()
		r.Post("/update", h.HandleUpdateJSON)
		r.Post("/update/", h.HandleUpdateJSON)
	}

	// Роуты для получения и обновления метрик
	r.Post("/value", h.HandleGetMetricJSON)
	r.Post("/value/", h.HandleGetMetricJSON)
	r.Post("/update/{type}/{name}/{value}", h.HandleUpdate)
	r.Post("/updates/", h.HandlerUpdateBatchJSON)
	r.Get("/value/{type}/{name}", h.HandleGetMetricValue)
	r.Get("/ping", h.HandlePing)
	r.Get("/", h.HandleMetricsPage)

	return r
}
