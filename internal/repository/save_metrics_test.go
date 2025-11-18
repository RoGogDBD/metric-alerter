package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	models "github.com/RoGogDBD/metric-alerter/internal/model"
	"github.com/stretchr/testify/require"
)

// TestGetEnvOrFlag_TableDriven выполняет табличные тесты для функций получения значения из переменной окружения или флага.
//
// Проверяет корректность работы функций GetEnvOrFlagInt, GetEnvOrFlagString и GetEnvOrFlagBool:
// - Если переменная окружения не установлена, возвращается значение по умолчанию.
// - Если переменная окружения установлена, возвращается её значение (с приведением типа).
//
// t — указатель на структуру теста.
func TestGetEnvOrFlag_TableDriven(t *testing.T) {
	t.Run("Int override by env", func(t *testing.T) {
		key := "TEST_SAVE_METRICS_INT"
		_ = os.Unsetenv(key)
		if got := GetEnvOrFlagInt(key, 42); got != 42 {
			t.Fatalf("expected default 42, got %d", got)
		}
		_ = os.Setenv(key, "7")
		defer func() { _ = os.Unsetenv(key) }()
		if got := GetEnvOrFlagInt(key, 42); got != 7 {
			t.Fatalf("expected env 7, got %d", got)
		}
	})

	t.Run("String override by env", func(t *testing.T) {
		key := "TEST_SAVE_METRICS_STR"
		_ = os.Unsetenv(key)
		if got := GetEnvOrFlagString(key, "def"); got != "def" {
			t.Fatalf("expected default def, got %q", got)
		}
		_ = os.Setenv(key, "envv")
		defer func() { _ = os.Unsetenv(key) }()
		if got := GetEnvOrFlagString(key, "def"); got != "envv" {
			t.Fatalf("expected env envv, got %q", got)
		}
	})

	t.Run("Bool override by env", func(t *testing.T) {
		key := "TEST_SAVE_METRICS_BOOL"
		_ = os.Unsetenv(key)
		if got := GetEnvOrFlagBool(key, true); got != true {
			t.Fatalf("expected default true, got %v", got)
		}
		_ = os.Setenv(key, "false")
		defer func() { _ = os.Unsetenv(key) }()
		if got := GetEnvOrFlagBool(key, true); got != false {
			t.Fatalf("expected env false, got %v", got)
		}
	})
}

// TestSaveAndLoadMetrics_TableDriven выполняет табличные тесты для функций сохранения и загрузки метрик.
//
// Для каждого теста:
// - Заполняет хранилище метрик с помощью функции setup.
// - Сохраняет метрики в файл с помощью SaveMetricsToFile.
// - Проверяет, что файл содержит корректный JSON.
// - Загружает метрики из файла в новое хранилище с помощью LoadMetricsFromFile.
// - Сравнивает исходные и загруженные метрики по количеству, типу и значению.
//
// t — указатель на структуру теста.
func TestSaveAndLoadMetrics_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		setup func(s Storage)
	}{
		{
			name: "gauge and counter",
			setup: func(s Storage) {
				s.SetGauge("gA", 1.5)
				s.AddCounter("cA", 10)
			},
		},
		{
			name: "only gauge",
			setup: func(s Storage) {
				s.SetGauge("gOnly", 2.25)
			},
		},
		{
			name: "only counter",
			setup: func(s Storage) {
				s.AddCounter("cOnly", 7)
			},
		},
		{
			name:  "empty storage",
			setup: func(s Storage) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			s := NewMemStorage()
			if tt.setup != nil {
				tt.setup(s)
			}

			fpath := filepath.Join(t.TempDir(), "metrics.json")
			require.NoError(t, SaveMetricsToFile(s, fpath))

			b, err := os.ReadFile(fpath)
			require.NoError(t, err)
			var arr []models.Metrics
			require.NoError(t, json.Unmarshal(b, &arr))

			s2 := NewMemStorage()
			require.NoError(t, LoadMetricsFromFile(s2, fpath))

			orig := s.GetAll()
			loaded := s2.GetAll()

			om := map[string]MetricInfo{}
			for _, mi := range orig {
				om[mi.Name] = mi
			}
			lm := map[string]MetricInfo{}
			for _, mi := range loaded {
				lm[mi.Name] = mi
			}

			require.Equal(t, len(orig), len(loaded))
			for k, omi := range om {
				lmi, ok := lm[k]
				require.True(t, ok, "missing key %s in loaded storage", k)
				require.Equal(t, omi.Type, lmi.Type)
				require.Equal(t, omi.Value, lmi.Value)
			}
		})
	}
}
