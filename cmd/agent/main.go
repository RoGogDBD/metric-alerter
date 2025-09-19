package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/RoGogDBD/metric-alerter/internal/config"
	models "github.com/RoGogDBD/metric-alerter/internal/model"
	"github.com/go-resty/resty/v2"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type Metric struct {
	Type  string
	Value float64
}

type MetricsSender interface {
	SendBatch(metrics []models.Metrics) error
}

type AgentState struct {
	PollInterval   int
	ReportInterval int
	PollCount      int64
	Metrics        map[string]Metric
	Rng            *rand.Rand
	Sender         MetricsSender
	Key            string
	mu             sync.RWMutex
	RateLimit      int
	jobQueue       chan []models.Metrics
}

func collectMetrics(state *AgentState) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := map[string]float64{
		"Alloc":         float64(m.Alloc),
		"BuckHashSys":   float64(m.BuckHashSys),
		"Frees":         float64(m.Frees),
		"GCCPUFraction": m.GCCPUFraction,
		"GCSys":         float64(m.GCSys),
		"HeapAlloc":     float64(m.HeapAlloc),
		"HeapIdle":      float64(m.HeapIdle),
		"HeapInuse":     float64(m.HeapInuse),
		"HeapObjects":   float64(m.HeapObjects),
		"HeapReleased":  float64(m.HeapReleased),
		"HeapSys":       float64(m.HeapSys),
		"LastGC":        float64(m.LastGC),
		"Lookups":       float64(m.Lookups),
		"MCacheInuse":   float64(m.MCacheInuse),
		"MCacheSys":     float64(m.MCacheSys),
		"MSpanInuse":    float64(m.MSpanInuse),
		"MSpanSys":      float64(m.MSpanSys),
		"Mallocs":       float64(m.Mallocs),
		"NextGC":        float64(m.NextGC),
		"NumForcedGC":   float64(m.NumForcedGC),
		"NumGC":         float64(m.NumGC),
		"OtherSys":      float64(m.OtherSys),
		"PauseTotalNs":  float64(m.PauseTotalNs),
		"StackInuse":    float64(m.StackInuse),
		"StackSys":      float64(m.StackSys),
		"Sys":           float64(m.Sys),
		"TotalAlloc":    float64(m.TotalAlloc),
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	for k, v := range metrics {
		state.Metrics[k] = Metric{"gauge", v}
	}

	state.PollCount++
	state.Metrics["PollCount"] = Metric{"counter", float64(state.PollCount)}
	state.Metrics["RandomValue"] = Metric{"gauge", state.Rng.Float64() * 100}
}

func collectSystemMetrics(state *AgentState) {
	vm, err := mem.VirtualMemory()
	if err == nil {
		state.mu.Lock()
		state.Metrics["TotalMemory"] = Metric{"gauge", float64(vm.Total)}
		state.Metrics["FreeMemory"] = Metric{"gauge", float64(vm.Free)}
		state.mu.Unlock()
	}

	if percents, err := cpu.Percent(0, true); err == nil {
		state.mu.Lock()
		for i, p := range percents {
			key := fmt.Sprintf("CPUutilization%d", i+1)
			state.Metrics[key] = Metric{"gauge", p}
		}
		state.mu.Unlock()
	}
}

func buildBatchSnapshot(state *AgentState) []models.Metrics {
	state.mu.RLock()
	defer state.mu.RUnlock()

	batch := make([]models.Metrics, 0, len(state.Metrics))
	for name, metric := range state.Metrics {
		m := models.Metrics{
			ID:    name,
			MType: metric.Type,
		}
		if metric.Type == "gauge" {
			val := metric.Value
			m.Value = &val
		} else {
			delta := int64(metric.Value)
			m.Delta = &delta
		}
		batch = append(batch, m)
	}
	return batch
}

func sendMetrics(state *AgentState) {
	batch := buildBatchSnapshot(state)
	if len(batch) == 0 {
		return
	}
	if err := state.Sender.SendBatch(batch); err != nil {
		log.Printf("Failed to send metrics batch: %v", err)
	}
}

func startWorkerPool(state *AgentState) {
	if state.RateLimit <= 0 {
		state.RateLimit = 1
	}
	state.jobQueue = make(chan []models.Metrics, state.RateLimit*2)

	for i := 0; i < state.RateLimit; i++ {
		go func(id int) {
			for batch := range state.jobQueue {
				if err := state.Sender.SendBatch(batch); err != nil {
					log.Printf("worker %d: send error: %v", id, err)
				}
			}
		}(i + 1)
	}
}

type RestySender struct {
	Client *resty.Client
	Key    string
}

func (rs *RestySender) SendBatch(metrics []models.Metrics) error {
	body, err := json.Marshal(metrics)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(body); err != nil {
		return fmt.Errorf("failed to write gzip: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return config.RetryWithBackoff(ctx, func() error {
		req := rs.Client.R().
			SetHeader("Content-Type", "application/json").
			SetHeader("Content-Encoding", "gzip").
			SetBody(buf.Bytes())

		if rs.Key != "" {
			hash := computeHash(buf.Bytes(), rs.Key)
			req.SetHeader("HashSHA256", hash)
		}

		resp, err := req.Post("/updates/")
		if err != nil {
			return fmt.Errorf("failed to POST metrics batch: %w", err)
		}
		if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("unexpected status: %d", resp.StatusCode())
		}
		return nil
	})
}

func computeHash(data []byte, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func parseFlags() (*config.NetAddress, *AgentState) {
	addr := config.ParseAddressFlag()
	poll := flag.Int("p", 2, "Poll interval in seconds")
	report := flag.Int("r", 10, "Report interval in seconds")
	key := flag.String("k", "", "Key for signing requests")
	limit := flag.Int("l", 1, "Rate limit (max concurrent outgoing requests)")

	flag.Parse()

	if val, err := config.EnvInt("POLL_INTERVAL"); err == nil && val != 0 {
		*poll = val
	}
	if val, err := config.EnvInt("REPORT_INTERVAL"); err == nil && val != 0 {
		*report = val
	}
	if val, err := config.EnvInt("RATE_LIMIT"); err == nil && val != 0 {
		*limit = val
	}

	keyValue := config.EnvString("KEY")
	if keyValue == "" {
		keyValue = *key
	}

	state := &AgentState{
		PollInterval:   *poll,
		ReportInterval: *report,
		PollCount:      0,
		Metrics:        make(map[string]Metric),
		Rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
		Key:            keyValue,
		RateLimit:      *limit,
	}

	return addr, state
}

func main() {
	addr, state := parseFlags()

	if err := config.EnvServer(addr, "ADDRESS"); err != nil {
		log.Fatalf("failed to apply env override: %v", err)
	}

	fmt.Println("Server URL", addr.String())
	fmt.Println("Report interval", state.ReportInterval)
	fmt.Println("Poll interval", state.PollInterval)

	restyClient := resty.New().
		SetBaseURL("http://" + addr.String()).
		SetTimeout(5 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(500 * time.Millisecond)

	state.Sender = &RestySender{Client: restyClient, Key: state.Key}

	startWorkerPool(state)

	go func(pollSec int) {
		t := time.NewTicker(time.Duration(pollSec) * time.Second)
		defer t.Stop()
		for range t.C {
			collectMetrics(state)
		}
	}(state.PollInterval)

	go func(pollSec int) {
		t := time.NewTicker(time.Duration(pollSec) * time.Second)
		defer t.Stop()
		for range t.C {
			collectSystemMetrics(state)
		}
	}(state.PollInterval)

	reportTicker := time.NewTicker(time.Duration(state.ReportInterval) * time.Second)
	defer reportTicker.Stop()

	for range reportTicker.C {
		batch := buildBatchSnapshot(state)
		if len(batch) == 0 {
			continue
		}
		state.jobQueue <- batch
	}
}
