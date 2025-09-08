package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/RoGogDBD/metric-alerter/internal/config"
	"github.com/RoGogDBD/metric-alerter/internal/config/db"
	"github.com/RoGogDBD/metric-alerter/internal/handler"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/RoGogDBD/metric-alerter/internal/service"
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

	var dbPool *pgxpool.Pool
	if dsn != "" {
		dbPool, err = db.InitDB(context.Background(), dsn)
		if err != nil {
			return err
		}
	} else {
		log.Println("No DSN provided, database features disabled")
	}

	storeInterval := repository.GetEnvOrFlagInt("STORE_INTERVAL", *storeIntervalFlag)
	fileStoragePath := repository.GetEnvOrFlagString("FILE_STORAGE_PATH", *fileStorageFlag)
	restore := repository.GetEnvOrFlagBool("RESTORE", *restoreFlag)

	storage := repository.NewMemStorage()
	handler := handler.NewHandler(storage, dbPool)

	if restore {
		if err := repository.LoadMetricsFromFile(storage, fileStoragePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to restore metrics: %v", err)
		}
	}

	r := service.NewRouter(handler, storage, storeInterval, fileStoragePath, logger)

	if err := config.EnvServer(addr, "ADDRESS"); err != nil {
		return err
	}

	log.Printf("Using address: %s\n", addr.String())
	fmt.Println("Server started")
	return http.ListenAndServe(addr.String(), r)
}
