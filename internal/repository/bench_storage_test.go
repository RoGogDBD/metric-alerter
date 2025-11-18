package repository

import (
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"testing"
)

func maybeWriteHeapProfile(b *testing.B) {
	out := os.Getenv("PROFILE_OUT")
	if out == "" {
		return
	}
	runtime.GC()
	f, err := os.Create(out)
	if err != nil {
		b.Fatalf("failed to create profile %s: %v", out, err)
	}
	defer func() { _ = f.Close() }()
	if err := pprof.WriteHeapProfile(f); err != nil {
		b.Fatalf("failed to write heap profile: %v", err)
	}
}

func BenchmarkMemStorage_SetGet(b *testing.B) {
	s := NewMemStorage()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := "metric" + strconv.Itoa(i%1000)
		s.SetGauge(name, float64(i))
		_, _ = s.GetGauge(name)
		s.AddCounter(name, int64(i%10))
		_, _ = s.GetCounter(name)
	}
	b.StopTimer()
	maybeWriteHeapProfile(b)
}
