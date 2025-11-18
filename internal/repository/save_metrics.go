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

func GetEnvOrFlagInt(envKey string, flagVal int) int {
	if v := os.Getenv(envKey); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return flagVal
}

func GetEnvOrFlagString(envKey string, flagVal string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return flagVal
}

func GetEnvOrFlagBool(envKey string, flagVal bool) bool {
	if v := os.Getenv(envKey); v != "" {
		return v == "true"
	}
	return flagVal
}

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
