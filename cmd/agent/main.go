package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"time"

	"github.com/RoGogDBD/metric-alerter/internal/config"
	models "github.com/RoGogDBD/metric-alerter/internal/model"
	"github.com/go-resty/resty/v2"
)

type Metric struct {
	Type  string
	Value float64
}

type MetricsSender interface {
	SendMetric(mType, mName string, mValue float64) error
}

type AgentState struct {
	PollInterval   int
	ReportInterval int
	PollCount      int64
	Metrics        map[string]Metric
	Rng            *rand.Rand
	Sender         MetricsSender
}

func collectMetrics(state *AgentState) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	state.Metrics["Alloc"] = Metric{"gauge", float64(m.Alloc)}
	state.Metrics["BuckHashSys"] = Metric{"gauge", float64(m.BuckHashSys)}
	state.Metrics["Frees"] = Metric{"counter", float64(m.Frees)}
	state.Metrics["GCCPUFraction"] = Metric{"gauge", m.GCCPUFraction}
	state.Metrics["GCSys"] = Metric{"gauge", float64(m.GCSys)}
	state.Metrics["HeapAlloc"] = Metric{"gauge", float64(m.HeapAlloc)}
	state.Metrics["HeapIdle"] = Metric{"gauge", float64(m.HeapIdle)}
	state.Metrics["HeapInuse"] = Metric{"gauge", float64(m.HeapInuse)}
	state.Metrics["HeapObjects"] = Metric{"gauge", float64(m.HeapObjects)}
	state.Metrics["HeapReleased"] = Metric{"gauge", float64(m.HeapReleased)}
	state.Metrics["HeapSys"] = Metric{"gauge", float64(m.HeapSys)}
	state.Metrics["LastGC"] = Metric{"gauge", float64(m.LastGC)}
	state.Metrics["Lookups"] = Metric{"counter", float64(m.Lookups)}
	state.Metrics["MCacheInuse"] = Metric{"gauge", float64(m.MCacheInuse)}
	state.Metrics["MCacheSys"] = Metric{"gauge", float64(m.MCacheSys)}
	state.Metrics["MSpanInuse"] = Metric{"gauge", float64(m.MSpanInuse)}
	state.Metrics["MSpanSys"] = Metric{"gauge", float64(m.MSpanSys)}
	state.Metrics["Mallocs"] = Metric{"counter", float64(m.Mallocs)}
	state.Metrics["NextGC"] = Metric{"gauge", float64(m.NextGC)}
	state.Metrics["NumForcedGC"] = Metric{"counter", float64(m.NumForcedGC)}
	state.Metrics["NumGC"] = Metric{"counter", float64(m.NumGC)}
	state.Metrics["OtherSys"] = Metric{"gauge", float64(m.OtherSys)}
	state.Metrics["PauseTotalNs"] = Metric{"counter", float64(m.PauseTotalNs)}
	state.Metrics["StackInuse"] = Metric{"gauge", float64(m.StackInuse)}
	state.Metrics["StackSys"] = Metric{"gauge", float64(m.StackSys)}
	state.Metrics["Sys"] = Metric{"gauge", float64(m.Sys)}
	state.Metrics["TotalAlloc"] = Metric{"gauge", float64(m.TotalAlloc)}
	state.Metrics["PollCount"] = Metric{"counter", float64(state.PollCount)}
	state.Metrics["RandomValue"] = Metric{"gauge", state.Rng.Float64() * 100}

	state.PollCount++
}

func sendMetrics(state *AgentState) {
	for name, metric := range state.Metrics {
		err := state.Sender.SendMetric(metric.Type, name, metric.Value)
		if err != nil {
			log.Printf("Error sending metric %s: %v", name, err)
		}
	}
}

type RestySender struct {
	Client *resty.Client
}

func (rs *RestySender) SendMetric(mType, mName string, mValue float64) error {
	var metric map[string]interface{}

	switch mType {
	case models.Gauge:
		metric = map[string]interface{}{
			"id":    mName,
			"type":  mType,
			"value": mValue,
		}
	case models.Counter:
		metric = map[string]interface{}{
			"id":    mName,
			"type":  mType,
			"delta": int64(mValue),
		}
	default:
		return fmt.Errorf("unsupported metric type: %s", mType)
	}

	body, err := json.Marshal(metric)
	if err != nil {
		return err
	}

	gzBody, err := config.GzipCompress(body)
	if err != nil {
		return err
	}

	resp, err := rs.Client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Content-Encoding", "gzip").
		SetBody(gzBody).
		Post("/update")

	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode())
	}
	return nil
}

func parseFlags() (*config.NetAddress, *AgentState) {
	addr := config.ParseAddressFlag()
	poll := flag.Int("p", 2, "Poll interval in seconds")
	report := flag.Int("r", 10, "Report interval in seconds")

	flag.Parse()

	if val, err := config.EnvInt("POLL_INTERVAL"); err != nil {
		log.Printf("%v", err)
	} else if val != 0 {
		*poll = val
	}

	if val, err := config.EnvInt("REPORT_INTERVAL"); err != nil {
		log.Printf("%v", err)
	} else if val != 0 {
		*report = val
	}

	state := &AgentState{
		PollInterval:   *poll,
		ReportInterval: *report,
		PollCount:      0,
		Metrics:        make(map[string]Metric),
		Rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
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

	state.Sender = &RestySender{Client: restyClient}

	pollDur := time.Duration(state.PollInterval) * time.Second
	reportDur := time.Duration(state.ReportInterval) * time.Second

	pollTicker := time.NewTicker(pollDur)
	reportTicker := time.NewTicker(reportDur)
	defer pollTicker.Stop()
	defer reportTicker.Stop()

	for {
		select {
		case <-pollTicker.C:
			collectMetrics(state)
		case <-reportTicker.C:
			sendMetrics(state)
		}
	}
}
