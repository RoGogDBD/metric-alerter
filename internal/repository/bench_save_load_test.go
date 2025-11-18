package repository

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"testing"
)

// maybeWriteHeapProfileSave записывает профиль кучи в файл, если задана переменная окружения PROFILE_OUT.
//
// Если переменная окружения PROFILE_OUT не установлена, функция ничего не делает.
// В противном случае выполняет сборку мусора, создает файл по указанному пути и сохраняет в него профиль кучи.
// В случае ошибки завершает тест с фатальной ошибкой.
//
// b — указатель на структуру теста/бенчмарка.
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

// BenchmarkSaveLoadMetrics измеряет производительность операций сохранения и загрузки метрик в файл.
//
// Создает в памяти 1000 метрик (gauge и counter), затем в цикле сохраняет их в файл и загружает обратно.
// После завершения бенчмарка, при необходимости, записывает профиль кучи.
//
// b — указатель на структуру теста/бенчмарка.
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
