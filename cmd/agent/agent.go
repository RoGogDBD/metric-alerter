package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

type AgentState struct {
	PollInterval   int
	ReportInterval int
	PollCount      int64
	Metrics        map[string]Metric
	Rng            *rand.Rand
	Client         *resty.Client
}

type Metric struct {
	Type  string
	Value float64
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
		mType := metric.Type
		mValue := strconv.FormatFloat(metric.Value, 'f', -1, 64)

		params := map[string]string{
			"mName":  name,
			"mType":  mType,
			"mValue": mValue,
		}
		resp, err := state.Client.R().
			SetPathParams(params).
			SetHeader("Content-Type", "text/plain").
			Post("/update/{mType}/{mName}/{mValue}")

		if err != nil {
			log.Printf("Error sending metric %s: %v", name, err)
			continue
		}
		if resp.StatusCode() != http.StatusOK {
			log.Printf("Unexpected status for %s: %d", name, resp.StatusCode())
		}
	}
}

type NetAddress struct {
	Host string
	Port int
}

func (a NetAddress) String() string {
	return a.Host + ":" + strconv.Itoa(a.Port)
}

func (a *NetAddress) Set(s string) error {
	hp := strings.Split(s, ":")
	a.Host = hp[0]
	if len(hp) == 2 {
		port, err := strconv.Atoi(hp[1])
		if err != nil {
			return err
		}
		a.Port = port
	} else {
		a.Port = 8080
	}
	return nil
}

func parseFlags() (*NetAddress, *AgentState) {
	addr := &NetAddress{Host: "localhost", Port: 8080}
	poll := flag.Int("p", 2, "Poll interval in seconds")
	report := flag.Int("r", 10, "Report interval in seconds")
	flag.Var(addr, "a", "Net address host:port")
	flag.Parse()

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

	fmt.Println("Server URL", addr.String())
	fmt.Println("Report interval", state.ReportInterval)
	fmt.Println("Poll interval", state.PollInterval)

	state.Client = resty.New().
		SetBaseURL("http://" + addr.String()).
		SetTimeout(5 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(500 * time.Millisecond)

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
