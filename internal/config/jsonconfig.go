package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Константы для имен переменных окружения
const (
	EnvAddress        = "ADDRESS"
	EnvRestore        = "RESTORE"
	EnvStoreInterval  = "STORE_INTERVAL"
	EnvStoreFile      = "FILE_STORAGE_PATH"
	EnvDatabaseDSN    = "DATABASE_DSN"
	EnvCryptoKey      = "CRYPTO_KEY"
	EnvAuditFile      = "AUDIT_FILE"
	EnvAuditURL       = "AUDIT_URL"
	EnvKey            = "KEY"
	EnvPollInterval   = "POLL_INTERVAL"
	EnvReportInterval = "REPORT_INTERVAL"
	EnvRateLimit      = "RATE_LIMIT"
	EnvConfig         = "CONFIG"
)

// Константы для флагов командной строки
const (
	FlagAddress        = "a"
	FlagRestore        = "r"
	FlagStoreInterval  = "i"
	FlagStoreFile      = "f"
	FlagDatabaseDSN    = "d"
	FlagCryptoKey      = "crypto-key"
	FlagAuditFile      = "audit-file"
	FlagAuditURL       = "audit-url"
	FlagKey            = "k"
	FlagPollInterval   = "p"
	FlagReportInterval = "r"
	FlagRateLimit      = "l"
	FlagConfig         = "c"
)

// ServerJSONConfig представляет конфигурацию сервера в формате JSON.
type ServerJSONConfig struct {
	Address       string `json:"address"`        // ADDRESS или флаг -a
	Restore       *bool  `json:"restore"`        // RESTORE или флаг -r
	StoreInterval string `json:"store_interval"` // STORE_INTERVAL или флаг -i (в формате "1s")
	StoreFile     string `json:"store_file"`     // FILE_STORAGE_PATH или флаг -f
	DatabaseDSN   string `json:"database_dsn"`   // DATABASE_DSN или флаг -d
	CryptoKey     string `json:"crypto_key"`     // CRYPTO_KEY или флаг -crypto-key
	AuditFile     string `json:"audit_file"`     // AUDIT_FILE или флаг -audit-file
	AuditURL      string `json:"audit_url"`      // AUDIT_URL или флаг -audit-url
	Key           string `json:"key"`            // KEY или флаг -k
}

// AgentJSONConfig представляет конфигурацию агента в формате JSON.
type AgentJSONConfig struct {
	Address        string `json:"address"`         // ADDRESS или флаг -a
	ReportInterval string `json:"report_interval"` // REPORT_INTERVAL или флаг -r (в формате "1s")
	PollInterval   string `json:"poll_interval"`   // POLL_INTERVAL или флаг -p (в формате "1s")
	RateLimit      *int   `json:"rate_limit"`      // RATE_LIMIT или флаг -l
	CryptoKey      string `json:"crypto_key"`      // CRYPTO_KEY или флаг -crypto-key
	Key            string `json:"key"`             // KEY или флаг -k
}

// loadJSONConfig — обобщенная функция для загрузки JSON конфигурации.
func loadJSONConfig(filePath string, v interface{}) error {
	if filePath == "" {
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

// LoadServerJSONConfig загружает конфигурацию сервера из JSON файла.
//
// filePath — путь к JSON файлу конфигурации.
// Возвращает указатель на ServerJSONConfig или ошибку.
func LoadServerJSONConfig(filePath string) (*ServerJSONConfig, error) {
	cfg := &ServerJSONConfig{}
	if err := loadJSONConfig(filePath, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadAgentJSONConfig загружает конфигурацию агента из JSON файла.
//
// filePath — путь к JSON файлу конфигурации.
// Возвращает указатель на AgentJSONConfig или ошибку.
func LoadAgentJSONConfig(filePath string) (*AgentJSONConfig, error) {
	cfg := &AgentJSONConfig{}
	if err := loadJSONConfig(filePath, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ParseDuration парсит строку длительности в формате "1s", "1m", "1h" и возвращает количество секунд.
// Если строка пуста, возвращает 0 и nil.
func ParseDuration(s string) (int, error) {
	if s == "" {
		return 0, nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration: %w", err)
	}

	return int(d.Seconds()), nil
}

// GetConfigFilePathWithFlag получает путь к файлу конфигурации, учитывая явно переданный флаг.
// Используется после flag.Parse().
func GetConfigFilePathWithFlag(flagValue string) string {
	// Флаги имеют больший приоритет
	if flagValue != "" {
		return flagValue
	}
	// Затем проверяем переменную окружения
	return EnvString(EnvConfig)
}
