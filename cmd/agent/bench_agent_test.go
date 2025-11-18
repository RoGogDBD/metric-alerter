package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"

	"github.com/go-resty/resty/v2"
)

// maybeWriteHeapProfileAgent записывает heap-профиль в файл, если переменная окружения PROFILE_OUT задана.
// Используется для профилирования памяти во время выполнения бенчмарка.
// В случае ошибки завершает выполнение теста с фатальной ошибкой.
//
// b - указатель на структуру тестирования *testing.B.
func maybeWriteHeapProfileAgent(b *testing.B) {
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

// BenchmarkSendMetrics выполняет бенчмарк отправки метрик через HTTP.
// Создаёт тестовый HTTP-сервер, инициализирует состояние агента и отправляет метрики в цикле.
// После завершения бенчмарка, при необходимости, записывает heap-профиль.
//
// b - указатель на структуру тестирования *testing.B.
func BenchmarkSendMetrics(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := resty.New().SetBaseURL(ts.URL)
	state := &AgentState{
		Collector: &MetricsCollector{metrics: map[string]Metric{"m": {Type: "gauge", Value: 1.0}}},
		Config:    Config{ReportInterval: 1, PollInterval: 1, RateLimit: 1},
		Sender:    &RestySender{Client: client},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sendMetrics(state)
	}
	b.StopTimer()
	maybeWriteHeapProfileAgent(b)
}
