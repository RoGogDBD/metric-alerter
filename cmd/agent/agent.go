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
	metrics       = make(map[string]interface{})
	rng           = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func collectMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics["Alloc"] = float64(m.Alloc)
	metrics["BuckHashSys"] = float64(m.BuckHashSys)
	metrics["Frees"] = float64(m.Frees)
	metrics["GCCPUFraction"] = m.GCCPUFraction
	metrics["GCSys"] = float64(m.GCSys)
	metrics["HeapAlloc"] = float64(m.HeapAlloc)
	metrics["HeapIdle"] = float64(m.HeapIdle)
	metrics["HeapInuse"] = float64(m.HeapInuse)
	metrics["HeapObjects"] = float64(m.HeapObjects)
	metrics["HeapReleased"] = float64(m.HeapReleased)
	metrics["HeapSys"] = float64(m.HeapSys)
	metrics["LastGC"] = float64(m.LastGC)
	metrics["Lookups"] = float64(m.Lookups)
	metrics["MCacheInuse"] = float64(m.MCacheInuse)
	metrics["MCacheSys"] = float64(m.MCacheSys)
	metrics["MSpanInuse"] = float64(m.MSpanInuse)
	metrics["MSpanSys"] = float64(m.MSpanSys)
	metrics["Mallocs"] = float64(m.Mallocs)
	metrics["NextGC"] = float64(m.NextGC)
	metrics["NumForcedGC"] = float64(m.NumForcedGC)
	metrics["NumGC"] = float64(m.NumGC)
	metrics["OtherSys"] = float64(m.OtherSys)
	metrics["PauseTotalNs"] = float64(m.PauseTotalNs)
	metrics["StackInuse"] = float64(m.StackInuse)
	metrics["StackSys"] = float64(m.StackSys)
	metrics["Sys"] = float64(m.Sys)
	metrics["TotalAlloc"] = float64(m.TotalAlloc)
	metrics["PollCount"] = pollCount
	metrics["RandomValue"] = rng.Float64() * 100

	pollCount++
}

func sendMetrics(client *resty.Client) {
	for name, val := range metrics {
		params := map[string]string{
			"mName": name,
		}
		var mType, mValue string
		switch v := val.(type) {
		case float64:
			mType = "gauge"
			mValue = strconv.FormatFloat(v, 'f', -1, 64)
		case int64:
			mType = "counter"
			mValue = strconv.FormatInt(v, 10)
		default:
			log.Printf("Unknown type of metric %s: %T", name, val)
			continue
		}
		params["mType"] = mType
		params["mValue"] = mValue

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
