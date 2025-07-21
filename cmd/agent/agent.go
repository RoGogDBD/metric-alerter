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

var (
	pollDefault   int
	reportDefault int
	pollCount     int64
	metrics       = make(map[string]Metric)
	rng           = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type Metric struct {
	Type  string
	Value float64
}

func collectMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics["Alloc"] = Metric{"gauge", float64(m.Alloc)}
	metrics["BuckHashSys"] = Metric{"gauge", float64(m.BuckHashSys)}
	metrics["Frees"] = Metric{"counter", float64(m.Frees)}
	metrics["GCCPUFraction"] = Metric{"gauge", m.GCCPUFraction}
	metrics["GCSys"] = Metric{"gauge", float64(m.GCSys)}
	metrics["HeapAlloc"] = Metric{"gauge", float64(m.HeapAlloc)}
	metrics["HeapIdle"] = Metric{"gauge", float64(m.HeapIdle)}
	metrics["HeapInuse"] = Metric{"gauge", float64(m.HeapInuse)}
	metrics["HeapObjects"] = Metric{"gauge", float64(m.HeapObjects)}
	metrics["HeapReleased"] = Metric{"gauge", float64(m.HeapReleased)}
	metrics["HeapSys"] = Metric{"gauge", float64(m.HeapSys)}
	metrics["LastGC"] = Metric{"gauge", float64(m.LastGC)}
	metrics["Lookups"] = Metric{"counter", float64(m.Lookups)}
	metrics["MCacheInuse"] = Metric{"gauge", float64(m.MCacheInuse)}
	metrics["MCacheSys"] = Metric{"gauge", float64(m.MCacheSys)}
	metrics["MSpanInuse"] = Metric{"gauge", float64(m.MSpanInuse)}
	metrics["MSpanSys"] = Metric{"gauge", float64(m.MSpanSys)}
	metrics["Mallocs"] = Metric{"counter", float64(m.Mallocs)}
	metrics["NextGC"] = Metric{"gauge", float64(m.NextGC)}
	metrics["NumForcedGC"] = Metric{"counter", float64(m.NumForcedGC)}
	metrics["NumGC"] = Metric{"counter", float64(m.NumGC)}
	metrics["OtherSys"] = Metric{"gauge", float64(m.OtherSys)}
	metrics["PauseTotalNs"] = Metric{"counter", float64(m.PauseTotalNs)}
	metrics["StackInuse"] = Metric{"gauge", float64(m.StackInuse)}
	metrics["StackSys"] = Metric{"gauge", float64(m.StackSys)}
	metrics["Sys"] = Metric{"gauge", float64(m.Sys)}
	metrics["TotalAlloc"] = Metric{"gauge", float64(m.TotalAlloc)}
	metrics["PollCount"] = Metric{"counter", float64(pollCount)}
	metrics["RandomValue"] = Metric{"gauge", rng.Float64() * 100}

	pollCount++
}

func sendMetrics(client *resty.Client) {
	for name, metric := range metrics {
		mType := metric.Type
		mValue := strconv.FormatFloat(metric.Value, 'f', -1, 64)

		params := map[string]string{
			"mName":  name,
			"mType":  mType,
			"mValue": mValue,
		}
		resp, err := client.R().
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

func parseFlags() *NetAddress {
	addr := &NetAddress{Host: "localhost", Port: 8080}
	flag.Var(addr, "a", "Net address host:post")
	flag.IntVar(&reportDefault, "r", 10, "Report interval in seconds (default: 10)")
	flag.IntVar(&pollDefault, "p", 2, "Poll interval in seconds (default: 2)")
	flag.Parse()

	return addr
}

func main() {
	addr := parseFlags()

	fmt.Println("Server URL", addr.String())
	fmt.Println("Report interval", reportDefault)
	fmt.Println("Poll interval", pollDefault)
	pollDur := time.Duration(pollDefault) * time.Second
	reportDur := time.Duration(reportDefault) * time.Second

	client := resty.New().
		SetBaseURL("http://" + addr.String()).
		SetTimeout(5 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(500 * time.Millisecond)

	pollTicker := time.NewTicker(pollDur)
	reportTicker := time.NewTicker(reportDur)

	defer pollTicker.Stop()
	defer reportTicker.Stop()

	for {
		select {
		case <-pollTicker.C:
			collectMetrics()

		case <-reportTicker.C:
			sendMetrics(client)
		}
	}
}
