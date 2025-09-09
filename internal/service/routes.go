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

func NewRouter(h *handler.Handler, storage repository.Storage, storeInterval int, filePath string, logger *zap.Logger) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(config.RequestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	if storeInterval == 0 {
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

	r.Post("/value", h.HandleGetMetricJSON)
	r.Post("/value/", h.HandleGetMetricJSON)
	r.Post("/update/{type}/{name}/{value}", h.HandleUpdate)
	r.Post("/updates/", h.HandlerUpdateBatchJSON)
	r.Get("/value/{type}/{name}", h.HandleGetMetricValue)
	r.Get("/ping", h.HandlePing)
	r.Get("/", h.HandleMetricsPage)

	return r
}
