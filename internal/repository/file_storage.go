package repository

import (
	"encoding/json"
	"io"
	"os"
	"strconv"

	models "github.com/RoGogDBD/metric-alerter/internal/model"
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
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(out)
}

func LoadMetricsFromFile(storage Storage, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
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
