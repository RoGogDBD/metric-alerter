package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/RoGogDBD/metric-alerter/internal/config"
	"github.com/RoGogDBD/metric-alerter/internal/handler"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}

func run() error {
	if err := config.Initialize("info"); err != nil {
		return err
	}
	defer config.Log.Sync()

	storage := repository.NewMemStorage()
	handler := handler.NewHandler(storage)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(config.RequestLogger)
	r.Use(middleware.Recoverer)
	r.Use(config.GzipResponseMiddleware)

	r.Post("/update", handler.HandleUpdateJSON)
	r.Post("/update/{type}/{name}/{value}", handler.HandleUpdate)
	r.Post("/value", handler.HandleGetMetricJSON)
	r.Get("/value/{type}/{name}", handler.HandleGetMetricValue)
	r.Get("/", handler.HandleMetricsPage)

	addr := config.ParseAddressFlag()
	flag.Parse()

	if err := config.EnvServer(addr, "ADDRESS"); err != nil {
		return err
	}

	log.Printf("Using address: %s\n", addr.String())
	fmt.Println("Server started")
	return http.ListenAndServe(addr.String(), r)
}
