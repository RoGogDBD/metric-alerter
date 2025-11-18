package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestValidateMetricInput_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		typeStr     string
		nameStr     string
		valueStr    string
		expectsErr  bool
		expectsType string
		expectsInt  int64
		expectsFlt  float64
	}{
		{"gauge ok", "gauge", "m1", "12.34", false, "gauge", 0, 12.34},
		{"gauge bad", "gauge", "m1", "notfloat", true, "", 0, 0},
		{"counter ok", "counter", "c1", "10", false, "counter", 10, 0},
		{"counter bad", "counter", "c1", "badint", true, "", 0, 0},
		{"unknown type", "unknown", "x", "1", true, "", 0, 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			m, err := ValidateMetricInput(tt.typeStr, tt.nameStr, tt.valueStr)
			if tt.expectsErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, m)

			mu := m
			require.Equal(t, tt.expectsType, mu.Type)
			require.Equal(t, tt.nameStr, mu.Name)
			if mu.IntVal != nil {
				require.Equal(t, tt.expectsInt, *mu.IntVal)
			}
			if mu.FloatVal != nil {
				require.InDelta(t, tt.expectsFlt, *mu.FloatVal, 1e-9)
			}
		})
	}
}

func TestHandler_HashVerification_TableDriven(t *testing.T) {
	h := NewHandler(nil, (*pgxpool.Pool)(nil))

	tests := []struct {
		name      string
		key       string
		payload   []byte
		headHash  string
		expectsOk bool
	}{
		{"no key no hash", "", []byte("data"), "", true},
		{"no key with hash", "", []byte("data"), "something", true},
		{"with key correct hash", "secret", []byte("payload"), "", true},
		{"with key incorrect hash", "secret", []byte("payload"), "badhash", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			h.key = tt.key
			head := tt.headHash
			if tt.key != "" && head == "" && tt.expectsOk {
				head = h.computeHash(tt.payload)
			}
			ok := h.verifyHash(tt.payload, head)
			require.Equal(t, tt.expectsOk, ok)
		})
	}
}
