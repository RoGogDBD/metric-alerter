package config

import (
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

func Initialize(level string) error {
	if err := os.MkdirAll("./logs", 0755); err != nil {
		return err
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

	var err error
	Log, err = config.Build()
	if err != nil {
		return err
	}

	return nil
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.size += size
	return size, err
}

func RequestLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		h.ServeHTTP(sr, r)
		duration := time.Since(start)

		Log.Info("HTTP request",
			zap.String("method", r.Method),
			zap.String("url", r.RequestURI),
			zap.Int("status", sr.status),
			zap.Int("size", sr.size),
			zap.Duration("duration", duration),
			zap.String("remote_addr", r.RemoteAddr),
		)
	})
}
