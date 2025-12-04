// Package main реализует HTTP-сервер для сбора и хранения метрик.
//
// Сервер предоставляет REST API для работы с метриками двух типов:
//   - gauge: датчики (значения устанавливаются)
//   - counter: счётчики (значения увеличиваются)
//
// @host localhost:8080
// @BasePath /
//
// @schemes http
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/RoGogDBD/metric-alerter/internal/config"
	"github.com/RoGogDBD/metric-alerter/internal/config/db"
	"github.com/RoGogDBD/metric-alerter/internal/handler"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/RoGogDBD/metric-alerter/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// buildVersion — версия сборки приложения.
	buildVersion string
	// buildDate — дата сборки приложения.
	buildDate string
	// buildCommit — хеш коммита сборки.
	buildCommit string
)

// main — точка входа в приложение сервера метрик.
// Инициализирует и запускает сервер, логирует фатальные ошибки при запуске.
func main() {
	printBuildInfo()
	if err := run(); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}

// printBuildInfo выводит информацию о сборке приложения.
func printBuildInfo() {
	version := "N/A"
	if buildVersion != "" {
		version = buildVersion
	}
	date := "N/A"
	if buildDate != "" {
		date = buildDate
	}
	commit := "N/A"
	if buildCommit != "" {
		commit = buildCommit
	}

	fmt.Printf("Build version: %s\n", version)
	fmt.Printf("Build date: %s\n", date)
	fmt.Printf("Build commit: %s\n", commit)
}

// run выполняет основную инициализацию и запуск HTTP-сервера.
//
// Возвращает ошибку при неудаче запуска или инициализации.
func run() error {
	// Инициализация логгера с уровнем info
	logger, err := config.Initialize("info")
	if err != nil {
		return err
	}
	defer logger.Sync()

	// Определение флагов командной строки
	dsnFlag := flag.String("d", "", "PostgreSQL DSN")
	storeIntervalFlag := flag.Int("i", 300, "Store interval in seconds")
	fileStorageFlag := flag.String("f", "metrics.json", "File storage path")
	restoreFlag := flag.Bool("r", true, "Restore metrics from file at startup")
	keyFlag := flag.String("k", "", "Key for request signing verification")
	auditFileFlag := flag.String("audit-file", "", "Path to audit log file")
	auditURLFlag := flag.String("audit-url", "", "URL for remote audit server")
	addr := config.ParseAddressFlag()
	flag.Parse()

	// Получение значений из переменных окружения или флагов
	dsn := repository.GetEnvOrFlagString("DATABASE_DSN", *dsnFlag)
	key := repository.GetEnvOrFlagString("KEY", *keyFlag)
	auditFile := repository.GetEnvOrFlagString("AUDIT_FILE", *auditFileFlag)
	auditURL := repository.GetEnvOrFlagString("AUDIT_URL", *auditURLFlag)

	// Инициализация менеджера аудита и добавление наблюдателей
	auditManager := repository.NewAuditManager()
	if auditFile != "" {
		if !filepath.IsAbs(auditFile) {
			if wd, err := os.Getwd(); err == nil {
				auditFile = filepath.Join(wd, auditFile)
			}
		}

		fileObserver := repository.NewFileAuditObserver(auditFile)
		auditManager.Attach(fileObserver)
		log.Printf("Audit file observer enabled: %s", auditFile)
	}
	if auditURL != "" {
		httpObserver := repository.NewHTTPAuditObserver(auditURL)
		auditManager.Attach(httpObserver)
		log.Printf("Audit HTTP observer enabled: %s", auditURL)
	}

	// Инициализация пула соединений с БД, если указан DSN
	var dbPool *pgxpool.Pool
	if dsn != "" {
		dbPool, err = db.InitDB(context.Background(), dsn)
		if err != nil {
			return err
		}
	} else {
		log.Println("No DSN provided, database features disabled")
	}

	// Получение параметров хранения и восстановления метрик
	storeInterval := repository.GetEnvOrFlagInt("STORE_INTERVAL", *storeIntervalFlag)
	fileStoragePath := repository.GetEnvOrFlagString("FILE_STORAGE_PATH", *fileStorageFlag)
	restore := repository.GetEnvOrFlagBool("RESTORE", *restoreFlag)

	// Инициализация хранилища и обработчика HTTP-запросов
	storage := repository.NewMemStorage()
	handler := handler.NewHandler(storage, dbPool)
	handler.SetKey(key)
	handler.SetAuditManager(auditManager)

	// Восстановление метрик из файла при необходимости
	if restore {
		if err := repository.LoadMetricsFromFile(storage, fileStoragePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to restore metrics: %v", err)
		}
	}

	// Создание маршрутизатора HTTP-запросов
	r := service.NewRouter(handler, storage, storeInterval, fileStoragePath, logger)

	// Применение адреса сервера из переменной окружения, если задано
	if err := config.EnvServer(addr, "ADDRESS"); err != nil {
		return err
	}

	log.Printf("Using address: %s\n", addr.String())
	fmt.Println("Server started")
	return http.ListenAndServe(addr.String(), r)
}
