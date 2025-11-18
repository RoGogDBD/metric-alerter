package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	models "github.com/RoGogDBD/metric-alerter/internal/model"
)

// FileAuditObserver записывает события аудита в файл
type FileAuditObserver struct {
	filePath string
	mu       sync.Mutex
}

// NewFileAuditObserver создает новый FileAuditObserver
func NewFileAuditObserver(filePath string) *FileAuditObserver {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create audit directory: %v", err)
	}

	return &FileAuditObserver{filePath: filePath}
}

// OnAuditEvent обрабатывает событие аудита, записывая его в файл
func (f *FileAuditObserver) OnAuditEvent(event models.AuditEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	file, err := os.OpenFile(f.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit file: %w", err)
	}
	defer func() { _ = file.Close() }()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	return nil
}

// HTTPAuditObserver отправляет события аудита на удаленный сервер
type HTTPAuditObserver struct {
	url    string
	client *http.Client
}

// NewHTTPAuditObserver создает новый HTTPAuditObserver
func NewHTTPAuditObserver(url string) *HTTPAuditObserver {
	return &HTTPAuditObserver{
		url:    url,
		client: &http.Client{},
	}
}

// OnAuditEvent обрабатывает событие аудита, отправляя его на удаленный сервер
func (h *HTTPAuditObserver) OnAuditEvent(event models.AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	resp, err := h.client.Post(h.url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send audit event: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("audit server returned status %d", resp.StatusCode)
	}

	return nil
}

// AuditManager управляет наблюдателями и уведомляет их о событиях
type AuditManager struct {
	observers []models.AuditObserver
	mu        sync.RWMutex
}

// NewAuditManager создает новый AuditManager
func NewAuditManager() *AuditManager {
	return &AuditManager{
		observers: make([]models.AuditObserver, 0),
	}
}

// Attach добавляет наблюдателя
func (a *AuditManager) Attach(observer models.AuditObserver) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.observers = append(a.observers, observer)
}

// Detach удаляет наблюдателя
func (a *AuditManager) Detach(observer models.AuditObserver) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, obs := range a.observers {
		if obs == observer {
			a.observers = append(a.observers[:i], a.observers[i+1:]...)
			break
		}
	}
}

// Notify уведомляет всех наблюдателей о событии
func (a *AuditManager) Notify(event models.AuditEvent) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, observer := range a.observers {
		if err := observer.OnAuditEvent(event); err != nil {
			log.Printf("Audit observer error: %v", err)
		}
	}
}

// HasObservers проверяет, есть ли подключенные наблюдатели
func (a *AuditManager) HasObservers() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.observers) > 0
}
