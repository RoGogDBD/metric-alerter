package repository

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"testing"
)

func maybeWriteHeapProfileSave(b *testing.B) {
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

func BenchmarkSaveLoadMetrics(b *testing.B) {
	s := NewMemStorage()
	for i := 0; i < 1000; i++ {
		name := "m" + strconv.Itoa(i)
		s.SetGauge(name, float64(i))
		s.AddCounter(name, int64(i))
	}
	fpath := filepath.Join(b.TempDir(), "metrics.json")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SaveMetricsToFile(s, fpath)
		_ = LoadMetricsFromFile(NewMemStorage(), fpath)
	}
	b.StopTimer()
	maybeWriteHeapProfileSave(b)
}
