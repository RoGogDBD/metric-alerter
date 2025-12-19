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
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/RoGogDBD/metric-alerter/internal/config"
	"github.com/RoGogDBD/metric-alerter/internal/config/db"
	"github.com/RoGogDBD/metric-alerter/internal/crypto"
	"github.com/RoGogDBD/metric-alerter/internal/handler"
	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/RoGogDBD/metric-alerter/internal/service"
	"github.com/RoGogDBD/metric-alerter/internal/version"
	"github.com/jackc/pgx/v5/pgxpool"
)

// main — точка входа в приложение сервера метрик.
// Инициализирует и запускает сервер, логирует фатальные ошибки при запуске.
func main() {
	version.PrintBuildInfo()
	if err := run(); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}

// run выполняет основную инициализацию и запуск HTTP-сервера.
// Возвращает ошибку при неудаче запуска или инициализации.
func run() error {
	// Инициализация логгера с уровнем info
	logger, err := config.Initialize("info")
	if err != nil {
		return err
	}
	defer logger.Sync()

	// Определение флагов командной строки
	configFileFlag := flag.String(config.FlagConfig, "", "Path to JSON config file")
	dsnFlag := flag.String(config.FlagDatabaseDSN, "", "PostgreSQL DSN")
	storeIntervalFlag := flag.Int(config.FlagStoreInterval, 300, "Store interval in seconds")
	fileStorageFlag := flag.String(config.FlagStoreFile, "metrics.json", "File storage path")
	restoreFlag := flag.Bool(config.FlagRestore, true, "Restore metrics from file at startup")
	keyFlag := flag.String(config.FlagKey, "", "Key for request signing verification")
	cryptoKeyFlag := flag.String(config.FlagCryptoKey, "", "Path to private key for asymmetric decryption")
	auditFileFlag := flag.String(config.FlagAuditFile, "", "Path to audit log file")
	auditURLFlag := flag.String(config.FlagAuditURL, "", "URL for remote audit server")
	addr := config.ParseAddressFlag()
	flag.Parse()

	// Конфигурация из JSON файла.
	configFilePath := config.GetConfigFilePathWithFlag(*configFileFlag)
	jsonConfig, err := config.LoadServerJSONConfig(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to load JSON config: %w", err)
	}

	// ADDRESS.
	if configFilePath != "" && jsonConfig.Address != "" {
		if err := addr.Set(jsonConfig.Address); err != nil {
			return fmt.Errorf("invalid address in config: %w", err)
		}
	}

	// DATABASE_DSN
	dsn := repository.GetEnvOrFlagString(config.EnvDatabaseDSN, *dsnFlag)
	if dsn == "" && jsonConfig.DatabaseDSN != "" {
		dsn = jsonConfig.DatabaseDSN
	}

	// KEY
	key := repository.GetEnvOrFlagString(config.EnvKey, *keyFlag)
	if key == "" && jsonConfig.Key != "" {
		key = jsonConfig.Key
	}

	// CRYPTO_KEY
	cryptoKeyPath := repository.GetEnvOrFlagString(config.EnvCryptoKey, *cryptoKeyFlag)
	if cryptoKeyPath == "" && jsonConfig.CryptoKey != "" {
		cryptoKeyPath = jsonConfig.CryptoKey
	}

	// AUDIT_FILE
	auditFile := repository.GetEnvOrFlagString(config.EnvAuditFile, *auditFileFlag)
	if auditFile == "" && jsonConfig.AuditFile != "" {
		auditFile = jsonConfig.AuditFile
	}

	// AUDIT_URL
	auditURL := repository.GetEnvOrFlagString(config.EnvAuditURL, *auditURLFlag)
	if auditURL == "" && jsonConfig.AuditURL != "" {
		auditURL = jsonConfig.AuditURL
	}

	var privateKey *rsa.PrivateKey
	if cryptoKeyPath != "" {
		var err error
		privateKey, err = crypto.LoadPrivateKey(cryptoKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load private key: %w", err)
		}
	}

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

	// Инициализация пула соединений с БД, если указан DSN.
	var dbPool *pgxpool.Pool
	if dsn != "" {
		dbPool, err = db.InitDB(context.Background(), dsn)
		if err != nil {
			return err
		}
		defer dbPool.Close()
	} else {
		log.Println("No DSN provided, database features disabled")
	}

	// Получение параметров хранения и восстановления метрик.
	storeInterval := repository.GetEnvOrFlagInt(config.EnvStoreInterval, *storeIntervalFlag)
	fileStoragePath := repository.GetEnvOrFlagString(config.EnvStoreFile, *fileStorageFlag)
	restore := repository.GetEnvOrFlagBool(config.EnvRestore, *restoreFlag)

	// Применяем значения из JSON конфига.
	if configFilePath != "" {
		// STORE_INTERVAL
		if storeInterval == 300 && *storeIntervalFlag == 300 && config.EnvString(config.EnvStoreInterval) == "" && jsonConfig.StoreInterval != "" {
			if val, err := config.ParseDuration(jsonConfig.StoreInterval); err == nil && val != 0 {
				storeInterval = val
			}
		}

		// STORE_FILE
		if fileStoragePath == "metrics.json" && *fileStorageFlag == "metrics.json" && config.EnvString(config.EnvStoreFile) == "" && jsonConfig.StoreFile != "" {
			fileStoragePath = jsonConfig.StoreFile
		}

		// RESTORE
		if jsonConfig.Restore != nil {
			restore = *jsonConfig.Restore
		}
	}

	// Инициализация хранилища и обработчика HTTP-запросов.
	storage := repository.NewMemStorage()
	h := handler.NewHandler(storage, dbPool)
	h.SetKey(key)
	h.SetCryptoKey(privateKey)
	h.SetAuditManager(auditManager)

	// Восстановление метрик из файла при необходимости.
	if restore {
		if err := repository.LoadMetricsFromFile(storage, fileStoragePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to restore metrics: %v", err)
		}
	}

	// Создание маршрутизатора HTTP-запросов.
	r := service.NewRouter(h, storage, storeInterval, fileStoragePath, logger)

	// Применение адреса сервера из переменной окружения, если задано.
	if err := config.EnvServer(addr, config.EnvAddress); err != nil {
		return err
	}

	log.Printf("Using address: %s\n", addr.String())
	fmt.Println("Server started")

	// Создание HTTP сервера.
	srv := &http.Server{
		Addr:    addr.String(),
		Handler: r,
	}

	// Канал для сигналов завершения.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Канал для ошибок сервера.
	errChan := make(chan error, 1)

	// Запуск сервера в отдельной горутине.
	go func() {
		log.Printf("Server listening on %s\n", srv.Addr)
		errChan <- srv.ListenAndServe()
	}()

	// Ожидание либо сигнала завершения, либо ошибки сервера.
	select {
	case err := <-errChan:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-sigChan:
		log.Printf("Received signal: %v. Starting graceful shutdown...\n", sig)

		// Сохраняем все несохранённые метрики
		log.Println("Saving unsaved metrics to file...")
		if err := repository.SaveMetricsToFile(storage, fileStoragePath); err != nil {
			log.Printf("Warning: failed to save metrics: %v\n", err)
		} else {
			log.Println("Metrics saved successfully")
		}

		// Создаём контекст с таймаутом для graceful shutdown.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Закрываем сервер с graceful shutdown.
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Error during server shutdown: %v\n", err)
			return fmt.Errorf("server shutdown error: %w", err)
		}

		log.Println("Server shutdown complete")
	}

	return nil
}
