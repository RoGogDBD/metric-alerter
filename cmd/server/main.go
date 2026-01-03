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
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
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
	"google.golang.org/grpc"
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
func run() error {
	// Инициализация логгера.
	logger, err := config.Initialize("info")
	if err != nil {
		return err
	}
	defer logger.Sync()

	// Определение флагов командной строки.
	configFileFlag := flag.String(config.FlagConfig, "", "Path to JSON config file")
	dsnFlag := flag.String(config.FlagDatabaseDSN, "", "PostgreSQL DSN")
	storeIntervalFlag := flag.Int(config.FlagStoreInterval, 300, "Store interval in seconds")
	fileStorageFlag := flag.String(config.FlagStoreFile, "metrics.json", "File storage path")
	restoreFlag := flag.Bool(config.FlagRestore, true, "Restore metrics from file at startup")
	keyFlag := flag.String(config.FlagKey, "", "Key for request signing verification")
	cryptoKeyFlag := flag.String(config.FlagCryptoKey, "", "Path to private key for asymmetric decryption")
	auditFileFlag := flag.String(config.FlagAuditFile, "", "Path to audit log file")
	auditURLFlag := flag.String(config.FlagAuditURL, "", "URL for remote audit server")
	trustedSubnetFlag := flag.String(config.FlagTrustedSubnet, "", "Trusted subnet in CIDR format")
	grpcAddressFlag := flag.String(config.FlagGRPCAddress, "", "gRPC server address")
	addr := config.ParseAddressFlag()
	flag.Parse()

	// Получение базовых значений (Приоритет: ENV > Flag).
	dsn := repository.GetEnvOrFlagString(config.EnvDatabaseDSN, *dsnFlag)
	storeInterval := repository.GetEnvOrFlagInt(config.EnvStoreInterval, *storeIntervalFlag)
	fileStoragePath := repository.GetEnvOrFlagString(config.EnvStoreFile, *fileStorageFlag)
	restore := repository.GetEnvOrFlagBool(config.EnvRestore, *restoreFlag)
	key := repository.GetEnvOrFlagString(config.EnvKey, *keyFlag)
	cryptoKeyPath := repository.GetEnvOrFlagString(config.EnvCryptoKey, *cryptoKeyFlag)
	auditFile := repository.GetEnvOrFlagString(config.EnvAuditFile, *auditFileFlag)
	auditURL := repository.GetEnvOrFlagString(config.EnvAuditURL, *auditURLFlag)
	trustedSubnet := repository.GetEnvOrFlagString(config.EnvTrustedSubnet, *trustedSubnetFlag)
	grpcAddress := repository.GetEnvOrFlagString(config.EnvGRPCAddress, *grpcAddressFlag)

	// Загрузка JSON конфигурации и применение к параметрам (низший приоритет).
	configFilePath := config.GetConfigFilePathWithFlag(*configFileFlag)
	if configFilePath != "" {
		jsonConfig, err := config.LoadServerJSONConfig(configFilePath)
		if err != nil {
			log.Printf("Warning: failed to load JSON config: %v", err)
		} else if jsonConfig != nil {
			// Вызов нового метода, который заменяет ручные проверки.
			jsonConfig.ApplyToServer(
				addr, &dsn, &storeInterval, &fileStoragePath,
				&restore, &key, &cryptoKeyPath, &auditFile, &auditURL, &trustedSubnet, &grpcAddress,
			)
		}
	}

	// Пост-обработка: загрузка RSA ключа.
	var privateKey *rsa.PrivateKey
	if cryptoKeyPath != "" {
		var err error
		privateKey, err = crypto.LoadPrivateKey(cryptoKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load private key: %w", err)
		}
	}

	// Инициализация менеджера аудита.
	auditManager := repository.NewAuditManager()
	if auditFile != "" {
		if !filepath.IsAbs(auditFile) {
			if wd, err := os.Getwd(); err == nil {
				auditFile = filepath.Join(wd, auditFile)
			}
		}
		auditManager.Attach(repository.NewFileAuditObserver(auditFile))
		log.Printf("Audit file observer enabled: %s", auditFile)
	}
	if auditURL != "" {
		auditManager.Attach(repository.NewHTTPAuditObserver(auditURL))
		log.Printf("Audit HTTP observer enabled: %s", auditURL)
	}

	// Инициализация базы данных.
	var dbPool *pgxpool.Pool
	if dsn != "" {
		dbPool, err = db.InitDB(context.Background(), dsn)
		if err != nil {
			return err
		}
		defer dbPool.Close()
	}

	// Инициализация хранилища и обработчиков.
	storage := repository.NewMemStorage()
	h := handler.NewHandler(storage, dbPool)
	h.SetKey(key)
	h.SetCryptoKey(privateKey)
	h.SetAuditManager(auditManager)
	var trustedSubnetNet *net.IPNet
	if trustedSubnet != "" {
		_, subnet, err := net.ParseCIDR(trustedSubnet)
		if err != nil {
			return fmt.Errorf("invalid trusted subnet: %w", err)
		}
		trustedSubnetNet = subnet
		h.SetTrustedSubnet(subnet)
	}

	if restore {
		if err := repository.LoadMetricsFromFile(storage, fileStoragePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to restore metrics: %v", err)
		}
	}

	r := service.NewRouter(h, storage, storeInterval, fileStoragePath, logger)

	// Переменная окружения ADDRESS имеет наивысший приоритет.
	if err := config.EnvServer(addr, config.EnvAddress); err != nil {
		return err
	}

	// Запуск сервера и обработка сигналов.
	srv := &http.Server{
		Addr:    addr.String(),
		Handler: r,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	errChan := make(chan error, 2)
	go func() {
		log.Printf("Server listening on %s\n", srv.Addr)
		errChan <- srv.ListenAndServe()
	}()

	var grpcSrv *grpc.Server
	if grpcAddress != "" {
		listener, err := net.Listen("tcp", grpcAddress)
		if err != nil {
			return fmt.Errorf("failed to listen gRPC address: %w", err)
		}
		grpcSrv = grpc.NewServer(grpc.UnaryInterceptor(grpcserver.IPSubnetInterceptor(trustedSubnetNet)))
		proto.RegisterMetricsServer(grpcSrv, grpcserver.NewMetricsService(storage, dbPool))
		go func() {
			log.Printf("gRPC server listening on %s\n", grpcAddress)
			if err := grpcSrv.Serve(listener); err != nil {
				errChan <- fmt.Errorf("gRPC server error: %w", err)
			}
		}()
	}

	select {
	case err := <-errChan:
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-sigChan:
		log.Printf("Received signal: %v. Starting graceful shutdown...\n", sig)
		repository.SaveMetricsToFile(storage, fileStoragePath)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		return srv.Shutdown(ctx)
	}

	return nil
}
