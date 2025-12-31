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
	EnvTrustedSubnet  = "TRUSTED_SUBNET"
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
	FlagTrustedSubnet  = "t"
	FlagPollInterval   = "p"
	FlagReportInterval = "r"
	FlagRateLimit      = "l"
	FlagConfig         = "c"
)

type (
	// ServerJSONConfig представляет конфигурацию сервера в формате JSON.
	ServerJSONConfig struct {
		Address       string `json:"address"`        // ADDRESS или флаг -a
		Restore       *bool  `json:"restore"`        // RESTORE или флаг -r
		StoreInterval string `json:"store_interval"` // STORE_INTERVAL или флаг -i (в формате "1s")
		StoreFile     string `json:"store_file"`     // FILE_STORAGE_PATH или флаг -f
		DatabaseDSN   string `json:"database_dsn"`   // DATABASE_DSN или флаг -d
		CryptoKey     string `json:"crypto_key"`     // CRYPTO_KEY или флаг -crypto-key
		AuditFile     string `json:"audit_file"`     // AUDIT_FILE или флаг -audit-file
		AuditURL      string `json:"audit_url"`      // AUDIT_URL или флаг -audit-url
		Key           string `json:"key"`            // KEY или флаг -k
		TrustedSubnet string `json:"trusted_subnet"` // TRUSTED_SUBNET или флаг -t
	}

	// AgentJSONConfig представляет конфигурацию агента в формате JSON.
	AgentJSONConfig struct {
		Address        string `json:"address"`         // ADDRESS или флаг -a
		ReportInterval string `json:"report_interval"` // REPORT_INTERVAL или флаг -r (в формате "1s")
		PollInterval   string `json:"poll_interval"`   // POLL_INTERVAL или флаг -p (в формате "1s")
		RateLimit      *int   `json:"rate_limit"`      // RATE_LIMIT или флаг -l
		CryptoKey      string `json:"crypto_key"`      // CRYPTO_KEY или флаг -crypto-key
		Key            string `json:"key"`             // KEY или флаг -k
	}
)

func (jc *AgentJSONConfig) ApplyToAgent(
	poll *int,
	report *int,
	limit *int,
	key *string,
	crypto *string,
	addr *NetAddress,
) {
	if jc == nil {
		return
	}

	if jc.Address != "" && addr.String() == "localhost:8080" {
		_ = addr.Set(jc.Address)
	}

	// PollInterval.
	if *poll == 2 && jc.PollInterval != "" {
		if val, err := ParseDuration(jc.PollInterval); err == nil && val != 0 {
			*poll = val
		}
	}

	// ReportInterval.
	if *report == 10 && jc.ReportInterval != "" {
		if val, err := ParseDuration(jc.ReportInterval); err == nil && val != 0 {
			*report = val
		}
	}

	// RateLimit.
	if *limit == 1 && jc.RateLimit != nil {
		*limit = *jc.RateLimit
	}

	// Key.
	if *key == "" && jc.Key != "" {
		*key = jc.Key
	}

	// CryptoKey.
	if *crypto == "" && jc.CryptoKey != "" {
		*crypto = jc.CryptoKey
	}
}

// ApplyToServer применяет настройки из ServerJSONConfig к переданным параметрам,
func (jc *ServerJSONConfig) ApplyToServer(
	addr *NetAddress,
	dsn *string,
	storeInt *int,
	storeFile *string,
	restore *bool,
	key *string,
	crypto *string,
	auditFile *string,
	auditURL *string,
	trustedSubnet *string,
) {
	if jc == nil {
		return
	}

	if jc.Address != "" && addr.String() == "localhost:8080" {
		_ = addr.Set(jc.Address)
	}
	if *dsn == "" && jc.DatabaseDSN != "" {
		*dsn = jc.DatabaseDSN
	}
	if *storeInt == 300 && jc.StoreInterval != "" {
		if val, err := ParseDuration(jc.StoreInterval); err == nil && val != 0 {
			*storeInt = val
		}
	}
	if *storeFile == "metrics.json" && jc.StoreFile != "" {
		*storeFile = jc.StoreFile
	}
	if jc.Restore != nil {
		*restore = *jc.Restore
	}
	if *key == "" && jc.Key != "" {
		*key = jc.Key
	}
	if *crypto == "" && jc.CryptoKey != "" {
		*crypto = jc.CryptoKey
	}
	if *auditFile == "" && jc.AuditFile != "" {
		*auditFile = jc.AuditFile
	}
	if *auditURL == "" && jc.AuditURL != "" {
		*auditURL = jc.AuditURL
	}
	if *trustedSubnet == "" && jc.TrustedSubnet != "" {
		*trustedSubnet = jc.TrustedSubnet
	}
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
