package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/RoGogDBD/metric-alerter/internal/config"
	"github.com/RoGogDBD/metric-alerter/internal/handler"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}

func run() error {
	logger, err := config.Initialize("info")
	if err != nil {
		return err
	}
	defer logger.Sync()

	dsnFlag := flag.String("d", "", "PostgreSQL DSN")
	storeIntervalFlag := flag.Int("i", 300, "Store interval in seconds")
	fileStorageFlag := flag.String("f", "metrics.json", "File storage path")
	restoreFlag := flag.Bool("r", true, "Restore metrics from file at startup")
	addr := config.ParseAddressFlag()
	flag.Parse()

	dsn := repository.GetEnvOrFlagString("DATABASE_DSN", *dsnFlag)

	var db *pgxpool.Pool
	if dsn != "" {
		db, err = pgxpool.New(context.Background(), dsn)
		if err != nil {
			return fmt.Errorf("failed to connect to db: %w", err)
		}
		if err = db.Ping(context.Background()); err != nil {
			return fmt.Errorf("failed to ping db: %w", err)
		}
		log.Println("Connected to PostgreSQL")
	} else {
		log.Println("No DSN provided, database features disabled")
	}

	storeInterval := repository.GetEnvOrFlagInt("STORE_INTERVAL", *storeIntervalFlag)
	fileStoragePath := repository.GetEnvOrFlagString("FILE_STORAGE_PATH", *fileStorageFlag)
	restore := repository.GetEnvOrFlagBool("RESTORE", *restoreFlag)

	storage := repository.NewMemStorage()
	handler := handler.NewHandler(storage, db)

	if restore {
		if err := repository.LoadMetricsFromFile(storage, fileStoragePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to restore metrics: %v", err)
		}
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(config.RequestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	if storeInterval == 0 {
		r.Post("/update", func(w http.ResponseWriter, r *http.Request) {
			handler.HandleUpdateJSON(w, r)
			if err := repository.SaveMetricsToFile(storage, fileStoragePath); err != nil {
				log.Printf("Failed to save metrics: %v", err)
			}
		})
		r.Post("/update/", func(w http.ResponseWriter, r *http.Request) {
			handler.HandleUpdateJSON(w, r)
			if err := repository.SaveMetricsToFile(storage, fileStoragePath); err != nil {
				log.Printf("Failed to save metrics: %v", err)
			}
		})
	} else {
		go func() {
			ticker := time.NewTicker(time.Duration(storeInterval) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if err := repository.SaveMetricsToFile(storage, fileStoragePath); err != nil {
					log.Printf("Failed to save metrics: %v", err)
				}
			}
		}()
		r.Post("/update", handler.HandleUpdateJSON)
		r.Post("/update/", handler.HandleUpdateJSON)
	}

	r.Post("/value", handler.HandleGetMetricJSON)
	r.Post("/value/", handler.HandleGetMetricJSON)
	r.Post("/update/{type}/{name}/{value}", handler.HandleUpdate)
	r.Get("/value/{type}/{name}", handler.HandleGetMetricValue)
	r.Get("/", handler.HandleMetricsPage)

	if err := config.EnvServer(addr, "ADDRESS"); err != nil {
		return err
	}

	log.Printf("Using address: %s\n", addr.String())
	fmt.Println("Server started")
	return http.ListenAndServe(addr.String(), r)
}
