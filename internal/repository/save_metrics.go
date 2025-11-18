package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/RoGogDBD/metric-alerter/internal/config"
	models "github.com/RoGogDBD/metric-alerter/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetEnvOrFlagInt возвращает значение переменной окружения по ключу envKey как int,
// либо значение flagVal, если переменная не установлена или не может быть преобразована.
//
// envKey — имя переменной окружения.
// flagVal — значение по умолчанию.
//
// Возвращает int.
func GetEnvOrFlagInt(envKey string, flagVal int) int {
	if v := os.Getenv(envKey); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return flagVal
}

// GetEnvOrFlagString возвращает значение переменной окружения по ключу envKey как строку,
// либо значение flagVal, если переменная не установлена.
//
// envKey — имя переменной окружения.
// flagVal — значение по умолчанию.
//
// Возвращает строку.
func GetEnvOrFlagString(envKey string, flagVal string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return flagVal
}

// GetEnvOrFlagBool возвращает значение переменной окружения по ключу envKey как bool,
// либо значение flagVal, если переменная не установлена.
//
// Значение переменной окружения считается true, если оно равно "true".
//
// envKey — имя переменной окружения.
// flagVal — значение по умолчанию.
//
// Возвращает bool.
func GetEnvOrFlagBool(envKey string, flagVal bool) bool {
	if v := os.Getenv(envKey); v != "" {
		return v == "true"
	}
	return flagVal
}

// SaveMetricsToFile сохраняет все метрики из хранилища storage в файл filePath в формате JSON.
//
// storage — интерфейс хранилища метрик.
// filePath — путь к файлу для сохранения.
//
// Возвращает ошибку при неудаче записи.
func SaveMetricsToFile(storage Storage, filePath string) error {
	metrics := storage.GetAll()
	var out []models.Metrics
	for _, m := range metrics {
		switch m.Type {
		case "gauge":
			val, _ := strconv.ParseFloat(m.Value, 64)
			out = append(out, models.Metrics{
				ID:    m.Name,
				MType: "gauge",
				Value: &val,
			})
		case "counter":
			delta, _ := strconv.ParseInt(m.Value, 10, 64)
			out = append(out, models.Metrics{
				ID:    m.Name,
				MType: "counter",
				Delta: &delta,
			})
		}
	}
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	return enc.Encode(out)
}

// SyncToDB синхронизирует все метрики из хранилища storage с базой данных db.
//
// Использует транзакцию и стратегию повторов с экспоненциальной задержкой.
// Для каждой метрики выполняет UPSERT (insert/update) в таблицу metrics.
//
// ctx — контекст выполнения.
// storage — интерфейс хранилища метрик.
// db — пул соединений с PostgreSQL.
//
// Возвращает ошибку при неудаче синхронизации.
func SyncToDB(ctx context.Context, storage Storage, db *pgxpool.Pool) error {
	return config.RetryWithBackoff(ctx, func() error {
		metrics := storage.GetAll()

		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		stmt := `
						INSERT INTO metrics (id, type, delta, value)
						VALUES ($1, $2, $3, $4)
						ON CONFLICT (id) DO UPDATE
						SET type = EXCLUDED.type,
							delta = EXCLUDED.delta,
							value = EXCLUDED.value
					`

		for _, m := range metrics {
			switch m.Type {
			case "gauge":
				val, _ := strconv.ParseFloat(m.Value, 64)
				if _, err := tx.Exec(ctx, stmt, m.Name, "gauge", nil, val); err != nil {
					return fmt.Errorf("failed to insert gauge %s: %w", m.Name, err)
				}
			case "counter":
				delta, _ := strconv.ParseInt(m.Value, 10, 64)
				if _, err := tx.Exec(ctx, stmt, m.Name, "counter", delta, nil); err != nil {
					return fmt.Errorf("failed to insert counter %s: %w", m.Name, err)
				}
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		return nil
	})
}

// LoadMetricsFromFile загружает метрики из файла filePath в хранилище storage.
//
// Ожидает, что файл содержит массив метрик в формате JSON.
// Для каждой метрики вызывает соответствующий метод хранилища.
//
// storage — интерфейс хранилища метрик.
// filePath — путь к файлу для загрузки.
//
// Возвращает ошибку при неудаче чтения или декодирования.
func LoadMetricsFromFile(storage Storage, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	var metrics []models.Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return err
	}
	for _, m := range metrics {
		switch m.MType {
		case "gauge":
			if m.Value != nil {
				storage.SetGauge(m.ID, *m.Value)
			}
		case "counter":
			if m.Delta != nil {
				storage.AddCounter(m.ID, *m.Delta)
			}
		}
	}
	return nil
}
