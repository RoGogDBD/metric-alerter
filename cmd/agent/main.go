package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/RoGogDBD/metric-alerter/internal/config"
	"github.com/RoGogDBD/metric-alerter/internal/crypto"
	models "github.com/RoGogDBD/metric-alerter/internal/model"
	"github.com/RoGogDBD/metric-alerter/internal/proto"
	"github.com/RoGogDBD/metric-alerter/internal/version"
	"github.com/go-resty/resty/v2"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var (
	// gzipPool — пул для переиспользования gzip.Writer, чтобы уменьшить аллокации при сжатии данных.
	gzipPool = sync.Pool{
		New: func() interface{} {
			// создаём writer, привязанный к io.Discard — он будет Reset-ом перенастроен перед использованием
			return gzip.NewWriter(io.Discard)
		},
	}

	// bufPool — пул для переиспользования bytes.Buffer при формировании тела запроса.
	bufPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

type (
	// Metric — структура для хранения метрики (тип и значение).
	Metric struct {
		Type  string  // Тип метрики: "gauge" или "counter"
		Value float64 // Значение метрики
	}

	// MetricsSender — интерфейс для отправки батча метрик.
	MetricsSender interface {
		// SendBatch отправляет срез метрик на сервер.
		SendBatch(metrics []models.Metrics) error
	}

	// Config — конфигурация агента.
	Config struct {
		PollInterval   int            // Интервал опроса метрик (сек).
		ReportInterval int            // Интервал отправки метрик (сек).
		RateLimit      int            // Ограничение на количество параллельных отправок.
		Key            string         // Ключ для подписи запросов.
		CryptoKey      *rsa.PublicKey // Публичный ключ для асимметричного шифрования.
		GRPCAddress    string         // Адрес gRPC-сервера.
	}

	// MetricsCollector — сборщик метрик, хранит значения и счетчик опросов.
	MetricsCollector struct {
		metrics   map[string]Metric // Собранные метрики.
		pollCount int64             // Счетчик опросов.
		rng       *rand.Rand        // Генератор случайных чисел.
		mu        sync.RWMutex      // Мьютекс для конкурентного доступа.
	}

	// AgentState — состояние агента, включает конфиг, сборщик, отправителя и очередь заданий.
	AgentState struct {
		Config    Config                // Конфигурация агента.
		Collector *MetricsCollector     // Сборщик метрик.
		Sender    MetricsSender         // Отправитель метрик.
		jobQueue  chan []models.Metrics // Очередь заданий для отправки метрик.
		wg        sync.WaitGroup        // Группа ожидания для воркеров.
	}

	// RestySender реализует MetricsSender, отправляя метрики через resty.Client.
	RestySender struct {
		Client    *resty.Client  // HTTP-клиент.
		Key       string         // Ключ для подписи.
		CryptoKey *rsa.PublicKey // Публичный ключ для асимметричного шифрования.
		RealIP    string         // IP хоста агента.
	}

	// GRPCSender реализует MetricsSender, отправляя метрики через gRPC.
	GRPCSender struct {
		Client proto.MetricsClient // gRPC клиент метрик.
		Conn   *grpc.ClientConn    // gRPC соединение.
		RealIP string              // IP хоста агента.
	}
)

// collectMetrics собирает метрики из runtime и обновляет их в коллекторе.
//
// state — текущее состояние агента.
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

	state.Collector.mu.Lock()
	defer state.Collector.mu.Unlock()

	for k, v := range metrics {
		state.Collector.metrics[k] = Metric{"gauge", v}
	}

	state.Collector.pollCount++
	state.Collector.metrics["PollCount"] = Metric{"counter", float64(state.Collector.pollCount)}
	state.Collector.metrics["RandomValue"] = Metric{"gauge", state.Collector.rng.Float64() * 100}
}

// collectSystemMetrics собирает системные метрики (память, CPU) и обновляет их в коллекторе.
func (c *MetricsCollector) collectSystemMetrics() {
	updates := make(map[string]Metric)

	if vm, err := mem.VirtualMemory(); err == nil {
		updates["TotalMemory"] = Metric{"gauge", float64(vm.Total)}
		updates["FreeMemory"] = Metric{"gauge", float64(vm.Free)}
	}

	if percents, err := cpu.Percent(0, true); err == nil {
		for i, p := range percents {
			key := fmt.Sprintf("CPUutilization%d", i+1)
			updates[key] = Metric{"gauge", p}
		}
	}

	c.mu.Lock()
	for k, v := range updates {
		c.metrics[k] = v
	}
	c.mu.Unlock()
}

// buildBatchSnapshot формирует срез метрик для отправки (снимок текущего состояния).
//
// state — текущее состояние агента.
// Возвращает срез моделей метрик для отправки.
func buildBatchSnapshot(state *AgentState) []models.Metrics {
	state.Collector.mu.RLock()
	defer state.Collector.mu.RUnlock()

	batch := make([]models.Metrics, 0, len(state.Collector.metrics))
	for name, metric := range state.Collector.metrics {
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

// sendMetrics отправляет батч метрик через Sender.
//
// state — текущее состояние агента.
func sendMetrics(state *AgentState) {
	batch := buildBatchSnapshot(state)
	if len(batch) == 0 {
		return
	}
	if err := state.Sender.SendBatch(batch); err != nil {
		log.Printf("Failed to send metrics batch: %v", err)
	}
}

// startWorkerPool запускает пул воркеров для параллельной отправки метрик.
//
// state — текущее состояние агента.
func startWorkerPool(state *AgentState) {
	if state.Config.RateLimit <= 0 {
		state.Config.RateLimit = 1
	}

	state.jobQueue = make(chan []models.Metrics)

	for i := 0; i < state.Config.RateLimit; i++ {
		state.wg.Add(1)
		go func(id int) {
			defer state.wg.Done()
			for batch := range state.jobQueue {
				if err := state.Sender.SendBatch(batch); err != nil {
					log.Printf("worker %d: send error: %v", id, err)
				}
			}
		}(i + 1)
	}
}

// SendBatch сжимает, подписывает, шифрует и отправляет батч метрик на сервер.
//
// metrics — срез метрик для отправки.
// Возвращает ошибку при неудаче.
func (rs *RestySender) SendBatch(metrics []models.Metrics) error {
	body, err := json.Marshal(metrics)
	if err != nil {
		return err
	}

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()

	gz := gzipPool.Get().(*gzip.Writer)
	gz.Reset(buf)

	if _, err := gz.Write(body); err != nil {
		gz.Reset(io.Discard)
		gzipPool.Put(gz)
		buf.Reset()
		bufPool.Put(buf)
		return fmt.Errorf("failed to write gzip: %w", err)
	}
	if err := gz.Close(); err != nil {
		gz.Reset(io.Discard)
		gzipPool.Put(gz)
		buf.Reset()
		bufPool.Put(buf)
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Содержимое сжатого буфера.
	compressed := make([]byte, buf.Len())
	copy(compressed, buf.Bytes())

	var hashSignature string
	if rs.Key != "" {
		hashSignature = computeHMACSHA256(compressed, rs.Key)
	}

	// Шифруем сжатые данные, если задан публичный ключ.
	dataToSend := compressed
	if rs.CryptoKey != nil {
		encrypted, err := crypto.EncryptData(compressed, rs.CryptoKey)
		if err != nil {
			gz.Reset(io.Discard)
			gzipPool.Put(gz)
			buf.Reset()
			bufPool.Put(buf)
			return fmt.Errorf("failed to encrypt data: %w", err)
		}
		dataToSend = encrypted
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Выполняем POST с повторными попытками.
	err = config.RetryWithBackoff(ctx, func() error {
		req := rs.Client.R().
			SetHeader("Content-Type", "application/json").
			SetHeader("Content-Encoding", "gzip").
			SetBody(dataToSend)

		if rs.RealIP != "" {
			req.SetHeader("X-Real-IP", rs.RealIP)
		}

		if rs.CryptoKey != nil {
			req.SetHeader("X-Encrypted", "true")
		}

		if hashSignature != "" {
			req.SetHeader("HashSHA256", hashSignature)
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

	// Сбрасываем и возвращаем объекты в пул.
	gz.Reset(io.Discard)
	gzipPool.Put(gz)
	buf.Reset()
	bufPool.Put(buf)

	return err
}

// SendBatch отправляет батч метрик на gRPC сервер.
func (gs *GRPCSender) SendBatch(metrics []models.Metrics) error {
	req := &proto.UpdateMetricsRequest{Metrics: buildGRPCMetrics(metrics)}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return config.RetryWithBackoff(ctx, func() error {
		requestCtx := ctx
		if gs.RealIP != "" {
			requestCtx = metadata.AppendToOutgoingContext(ctx, "x-real-ip", gs.RealIP)
		}
		if _, err := gs.Client.UpdateMetrics(requestCtx, req); err != nil {
			return fmt.Errorf("failed to send metrics via gRPC: %w", err)
		}
		return nil
	})
}

// Close закрывает gRPC соединение.
func (gs *GRPCSender) Close() error {
	return gs.Conn.Close()
}

// resolveHostIP пытается определить IP-адрес хоста агента.
func resolveHostIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		ip = ip.To4()
		if ip == nil {
			continue
		}
		return ip.String()
	}

	return "127.0.0.1"
}

// computeHMACSHA256 вычисляет HMAC-SHA256 для данных с заданным ключом.
//
// data — данные для подписи.
// key — ключ для HMAC.
// Возвращает hex-строку подписи.
func computeHMACSHA256(data []byte, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// FIX.

// buildGRPCMetrics преобразует метрики агента в gRPC формат.
func buildGRPCMetrics(metrics []models.Metrics) []*proto.Metric {
	result := make([]*proto.Metric, 0, len(metrics))
	for _, m := range metrics {
		out := &proto.Metric{
			Id:   m.ID,
			Type: proto.Metric_GAUGE,
		}
		switch m.MType {
		case "counter":
			out.Type = proto.Metric_COUNTER
			if m.Delta != nil {
				out.Delta = *m.Delta
			}
		default:
			if m.Value != nil {
				out.Value = *m.Value
			}
		}
		result = append(result, out)
	}
	return result
}

// parseFlags парсит флаги командной строки и переменные окружения, возвращает адрес сервера и состояние агента.
//
// Возвращает указатель на сетевой адрес и состояние агента.
func parseFlags() (*config.NetAddress, *AgentState) {
	addr := config.ParseAddressFlag()
	configFileFlag := flag.String(config.FlagConfig, "", "Path to JSON config file")
	poll := flag.Int(config.FlagPollInterval, 2, "Poll interval in seconds")
	report := flag.Int(config.FlagReportInterval, 10, "Report interval in seconds")
	key := flag.String(config.FlagKey, "", "Key for signing requests")
	limit := flag.Int(config.FlagRateLimit, 1, "Rate limit (max concurrent outgoing requests)")
	cryptoKey := flag.String(config.FlagCryptoKey, "", "Path to public key for asymmetric encryption")
	grpcAddress := flag.String(config.FlagGRPCAddress, "", "gRPC server address")

	flag.Parse()

	if envPoll, err := config.EnvInt(config.EnvPollInterval); err == nil && envPoll != 0 {
		*poll = envPoll
	}
	if envReport, err := config.EnvInt(config.EnvReportInterval); err == nil && envReport != 0 {
		*report = envReport
	}
	if envLimit, err := config.EnvInt(config.EnvRateLimit); err == nil && envLimit != 0 {
		*limit = envLimit
	}

	if envKey := config.EnvString(config.EnvKey); envKey != "" {
		*key = envKey
	}
	if envCrypto := config.EnvString(config.EnvCryptoKey); envCrypto != "" {
		*cryptoKey = envCrypto
	}
	if envGRPC := config.EnvString(config.EnvGRPCAddress); envGRPC != "" {
		*grpcAddress = envGRPC
	}

	configFilePath := config.GetConfigFilePathWithFlag(*configFileFlag)
	if configFilePath != "" {
		jsonConfig, err := config.LoadAgentJSONConfig(configFilePath)
		if err != nil {
			log.Printf("Warning: failed to load JSON config: %v", err)
		} else if jsonConfig != nil {
			jsonConfig.ApplyToAgent(poll, report, limit, key, cryptoKey, addr, grpcAddress)
		}
	}

	var publicKey *rsa.PublicKey
	if *cryptoKey != "" {
		var err error
		publicKey, err = crypto.LoadPublicKey(*cryptoKey)
		if err != nil {
			log.Fatalf("failed to load public key: %v", err)
		}
	}

	state := &AgentState{
		Config: Config{
			PollInterval:   *poll,
			ReportInterval: *report,
			RateLimit:      *limit,
			Key:            *key,
			CryptoKey:      publicKey,
			GRPCAddress:    *grpcAddress,
		},
		Collector: &MetricsCollector{
			metrics:   make(map[string]Metric),
			pollCount: 0,
			rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
		},
	}

	return addr, state
}

// main — точка входа агента. Запускает сбор метрик, воркеры и отправку на сервер.
func main() {
	version.PrintBuildInfo()

	addr, state := parseFlags()

	if err := config.EnvServer(addr, config.EnvAddress); err != nil {
		log.Fatalf("failed to apply env override: %v", err)
	}

	fmt.Println("Server URL", addr.String())
	fmt.Println("Report interval", state.Config.ReportInterval)
	fmt.Println("Poll interval", state.Config.PollInterval)

	if state.Config.GRPCAddress != "" {
		conn, err := grpc.NewClient(
			state.Config.GRPCAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatalf("failed to connect to gRPC server: %v", err)
		}
		state.Sender = &GRPCSender{
			Client: proto.NewMetricsClient(conn),
			Conn:   conn,
			RealIP: resolveHostIP(),
		}
		log.Printf("gRPC sender enabled: %s", state.Config.GRPCAddress)
	} else {
		restyClient := resty.New().
			SetBaseURL("http://" + addr.String()).
			SetTimeout(5 * time.Second).
			SetRetryCount(3).
			SetRetryWaitTime(500 * time.Millisecond)

		state.Sender = &RestySender{
			Client:    restyClient,
			Key:       state.Config.Key,
			CryptoKey: state.Config.CryptoKey,
			RealIP:    resolveHostIP(),
		}
	}

	startWorkerPool(state)

	// Канал для сигналов завершения.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Запуск pprof-сервера для профилирования.
	go func() {
		log.Println("pprof http server listening on :6060")
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Printf("pprof server failed: %v", err)
		}
	}()

	// Периодический сбор метрик runtime.
	pollCtx, pollCancel := context.WithCancel(context.Background())
	go func(pollSec int) {
		t := time.NewTicker(time.Duration(pollSec) * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				collectMetrics(state)
			case <-pollCtx.Done():
				return
			}
		}
	}(state.Config.PollInterval)

	// Периодический сбор системных метрик.
	sysCtx, sysCancel := context.WithCancel(context.Background())
	go func(pollSec int) {
		t := time.NewTicker(time.Duration(pollSec) * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				state.Collector.collectSystemMetrics()
			case <-sysCtx.Done():
				return
			}
		}
	}(state.Config.PollInterval)

	// Периодическая отправка метрик с поддержкой graceful shutdown.
	reportTicker := time.NewTicker(time.Duration(state.Config.ReportInterval) * time.Second)
	defer reportTicker.Stop()

	log.Println("Agent started. Waiting for signals...")

	for {
		select {
		case <-reportTicker.C:
			batch := buildBatchSnapshot(state)
			if len(batch) == 0 {
				continue
			}
			state.jobQueue <- batch

		case sig := <-sigChan:
			log.Printf("Received signal: %v. Starting graceful shutdown...\n", sig)

			// Отправляем последний батч метрик.
			finalBatch := buildBatchSnapshot(state)
			if len(finalBatch) > 0 {
				log.Printf("Sending final batch of %d metrics...\n", len(finalBatch))
				state.jobQueue <- finalBatch
			}

			// Останавливаем горутины сбора метрик.
			pollCancel()
			sysCancel()

			// Закрываем очередь заданий.
			close(state.jobQueue)

			// Ждем завершения всех воркеров.
			log.Println("Waiting for pending requests to complete...")
			state.wg.Wait()

			if closer, ok := state.Sender.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					log.Printf("failed to close sender: %v", err)
				}
			}

			log.Println("Agent shutdown complete")
			return
		}
	}
}
