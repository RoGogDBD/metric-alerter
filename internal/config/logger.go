package config

import (
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Initialize инициализирует zap.Logger с заданным уровнем логирования.
//
// level — строка, определяющая уровень логирования ("debug", "warn", "error", по умолчанию "info").
// Логи пишутся в файл ./logs/app.log и в stdout. Время логируется в формате ISO8601.
//
// Возвращает инициализированный *zap.Logger или ошибку при неудаче.
func Initialize(level string) (*zap.Logger, error) {
	if err := os.MkdirAll("./logs", 0755); err != nil {
		return nil, err
	}
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{
		"./logs/app.log",
		"stdout",
	}

	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	lvl := zapcore.InfoLevel
	switch strings.ToLower(level) {
	case "debug":
		lvl = zapcore.DebugLevel
	case "warn":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	}
	config.Level = zap.NewAtomicLevelAt(lvl)

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// statusRecorder реализует http.ResponseWriter и позволяет сохранять статус и размер ответа.
//
// Используется для логирования HTTP-запросов с сохранением кода статуса и размера ответа.
type statusRecorder struct {
	http.ResponseWriter
	status int // HTTP-статус ответа
	size   int // Размер тела ответа в байтах
}

// WriteHeader сохраняет статус ответа и вызывает оригинальный WriteHeader.
func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Write записывает данные в ответ и увеличивает счетчик размера ответа.
func (r *statusRecorder) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.size += size
	return size, err
}

// RequestLogger возвращает middleware для логирования HTTP-запросов с помощью zap.Logger.
//
// Для каждого запроса логируются метод, URL, статус, размер ответа, длительность и удалённый адрес.
func RequestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			h.ServeHTTP(sr, r)
			duration := time.Since(start)

			logger.Info("HTTP request",
				zap.String("method", r.Method),
				zap.String("url", r.RequestURI),
				zap.Int("status", sr.status),
				zap.Int("size", sr.size),
				zap.Duration("duration", duration),
				zap.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}
