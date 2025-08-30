package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

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

	addr := config.ParseAddressFlag()
	dsnFlag := config.ParseDSNFlag()
	flag.Parse()

	dsn := *dsnFlag
	if dsn == "" {
		dsn = os.Getenv("DATABASE_DSN")
	}

	if err := config.EnvServer(addr, "ADDRESS"); err != nil {
		return err
	}

	var postgres *repository.Postgres
	if dsn != "" {
		var err error
		postgres, err = repository.NewPostgres(dsn)
		if err != nil {
			return fmt.Errorf("failed to connect postgres: %w", err)
		}
		defer postgres.Close()

		if err := postgres.Ping(context.Background()); err != nil {
			return fmt.Errorf("database ping failed: %w", err)
		}
		log.Println("Postgres connected")
	} else {
		log.Println("Postgres DSN not provided, running without database")
	}

	storage := repository.NewMemStorage()
	handler := handler.NewHandler(storage, postgres)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(config.RequestLogger)
	r.Use(middleware.Recoverer)

	r.Post("/update/{type}/{name}/{value}", handler.HandleUpdate)
	r.Get("/value/{type}/{name}", handler.HandleGetMetricValue)
	r.Get("/ping", handler.HandlePing)
	r.Get("/", handler.HandleMetricsPage)

	log.Printf("Using address: %s\n", addr.String())
	fmt.Println("Server started")
	return http.ListenAndServe(addr.String(), r)
}
