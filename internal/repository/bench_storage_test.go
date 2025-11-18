package repository

import (
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"testing"
)

// maybeWriteHeapProfile записывает профиль кучи в файл, если установлена переменная окружения PROFILE_OUT.
//
// Если переменная окружения PROFILE_OUT не задана, функция ничего не делает.
// В противном случае выполняет сборку мусора, создает файл по указанному пути и сохраняет в него профиль кучи.
// В случае ошибки завершает выполнение теста с фатальной ошибкой.
//
// b — указатель на структуру теста/бенчмарка.
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

// BenchmarkMemStorage_SetGet измеряет производительность операций установки и получения метрик в MemStorage.
//
// В цикле выполняет SetGauge, GetGauge, AddCounter и GetCounter для 1000 различных метрик.
// После завершения бенчмарка, при необходимости, записывает профиль кучи.
//
// b — указатель на структуру теста/бенчмарка.
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
