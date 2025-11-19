package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockNetAddr — мок-реализация интерфейса AddrSetter для тестирования.
// Позволяет эмулировать установку значения адреса и возвращать ошибку при необходимости.
type mockNetAddr struct {
	setValue string // Последнее установленное значение
	err      error  // Ошибка, которую нужно вернуть при вызове Set
}

// Set устанавливает значение адреса и возвращает ошибку, если она задана.
func (m *mockNetAddr) Set(val string) error {
	m.setValue = val
	return m.err
}

// TestEnvInt тестирует функцию EnvInt на корректность обработки различных значений переменных окружения.
//
// Проверяются следующие сценарии:
//   - Корректное целое число
//   - Пустое значение переменной окружения
//   - Некорректное (нечисловое) значение
func TestEnvInt(t *testing.T) {
	tests := []struct {
		name     string // Название теста
		envKey   string // Имя переменной окружения
		envValue string // Значение переменной окружения
		expected int    // Ожидаемое значение результата
		wantErr  bool   // Ожидается ли ошибка
	}{
		{
			name:     "valid integer",
			envKey:   "TEST_ENV_INT",
			envValue: "42",
			expected: 42,
			wantErr:  false,
		},
		{
			name:     "empty value",
			envKey:   "TEST_ENV_EMPTY",
			envValue: "",
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "invalid integer",
			envKey:   "TEST_ENV_INVALID",
			envValue: "notanint",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Устанавливаем или сбрасываем переменную окружения в зависимости от теста
			if tt.envValue != "" || tt.name == "empty value" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}

			got, err := EnvInt(tt.envKey)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, got)
			}
		})
	}
}

// TestEnvServer тестирует функцию EnvServer на корректность установки адреса из переменной окружения.
//
// Проверяются следующие сценарии:
//   - Корректное значение адреса
//   - Ошибка при установке адреса (мок возвращает ошибку)
//   - Переменная окружения не установлена
func TestEnvServer(t *testing.T) {
	tests := []struct {
		name      string // Название теста
		envKey    string // Имя переменной окружения
		envValue  string // Значение переменной окружения
		setErr    error  // Ошибка, которую должен вернуть Set
		expectErr bool   // Ожидается ли ошибка
	}{
		{
			name:      "valid address",
			envKey:    "ADDR_ENV",
			envValue:  "localhost:8080",
			setErr:    nil,
			expectErr: false,
		},
		{
			name:      "Set returns error",
			envKey:    "ADDR_ENV",
			envValue:  "invalid",
			setErr:    fmt.Errorf("bad addr"),
			expectErr: true,
		},
		{
			name:      "env var not set",
			envKey:    "ADDR_ENV",
			envValue:  "",
			setErr:    nil,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Устанавливаем или сбрасываем переменную окружения в зависимости от теста
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}

			mockAddr := &mockNetAddr{err: tt.setErr}

			err := EnvServer(mockAddr, tt.envKey)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
