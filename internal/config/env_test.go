package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockNetAddr struct {
	setValue string
	err      error
}

func (m *mockNetAddr) Set(val string) error {
	m.setValue = val
	return m.err
}

func TestEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		expected int
		wantErr  bool
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
			os.Setenv(tt.envKey, tt.envValue)
			defer os.Unsetenv(tt.envKey)

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

func TestEnvServer(t *testing.T) {
	tests := []struct {
		name      string
		envKey    string
		envValue  string
		setErr    error
		expectErr bool
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
