package agent

import (
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	pollInterval   = 2 * time.Second
	reportInterval = 10 * time.Second
)

var (
	metrics   = make(map[string]interface{})
	pollCount int64
	rng       = rand.New(rand.NewSource(time.Now().UnixNano()))
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
			log.Printf("Неизвестный тип метрики %s: %T", name, val)
			continue
		}
		params["mType"] = mType
		params["mValue"] = mValue

		resp, err := client.R().
			SetPathParams(params).
			SetHeader("Content-Type", "text/plain").
			Post("/update/{mType}/{mName}/{mValue}")
		if err != nil {
			log.Printf("Ошибка отправки метрики %s: %v", name, err)
			continue
		}
		if resp.StatusCode() != http.StatusOK {
			log.Printf("Неожиданный статус для %s: %d", name, resp.StatusCode())
		}
	}
}

func main() {
	client := resty.New().
		SetBaseURL("http://localhost:8080").
		SetTimeout(5 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(500 * time.Millisecond)

	pollTicker := time.NewTicker(pollInterval)
	reportTicker := time.NewTicker(reportInterval)

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
