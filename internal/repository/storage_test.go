package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemStorage_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		setup func(s Storage)
		check func(t *testing.T, s Storage)
	}{
		{
			name: "gauge set/get",
			setup: func(s Storage) {
				s.SetGauge("g1", 3.14)
			},
			check: func(t *testing.T, s Storage) {
				v, ok := s.GetGauge("g1")
				require.True(t, ok)
				require.InEpsilon(t, 3.14, v, 1e-9)
				_, ok2 := s.GetGauge("missing")
				require.False(t, ok2)
			},
		},
		{
			name: "counter add/get",
			setup: func(s Storage) {
				s.AddCounter("c1", 10)
				s.AddCounter("c1", 5)
			},
			check: func(t *testing.T, s Storage) {
				v, ok := s.GetCounter("c1")
				require.True(t, ok)
				require.Equal(t, int64(15), v)
				_, ok2 := s.GetCounter("missing")
				require.False(t, ok2)
			},
		},
		{
			name: "getall combined",
			setup: func(s Storage) {
				s.SetGauge("g2", 2.5)
				s.AddCounter("c2", 7)
			},
			check: func(t *testing.T, s Storage) {
				all := s.GetAll()
				m := map[string]MetricInfo{}
				for _, mi := range all {
					m[mi.Name] = mi
				}
				mi, ok := m["g2"]
				require.True(t, ok)
				require.Equal(t, "gauge", mi.Type)
				require.Equal(t, "2.5", mi.Value)
				mi2, ok := m["c2"]
				require.True(t, ok)
				require.Equal(t, "counter", mi2.Type)
				require.Equal(t, "7", mi2.Value)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			s := NewMemStorage()
			if tt.setup != nil {
				tt.setup(s)
			}
			if tt.check != nil {
				tt.check(t, s)
			}
		})
	}
}
