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

// FileAuditObserver записывает события аудита в файл.
//
// Поля:
//   - filePath: путь к файлу для записи событий
//   - mu: мьютекс для синхронизации доступа к файлу
type FileAuditObserver struct {
	filePath string
	mu       sync.Mutex
}

// NewFileAuditObserver создает новый экземпляр FileAuditObserver.
//
// filePath — путь к файлу аудита.
//
// Возвращает указатель на FileAuditObserver.
func NewFileAuditObserver(filePath string) *FileAuditObserver {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create audit directory: %v", err)
	}

	return &FileAuditObserver{filePath: filePath}
}

// OnAuditEvent обрабатывает событие аудита, записывая его в файл.
//
// event — событие аудита для записи.
//
// Возвращает ошибку при неудаче записи.
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

// HTTPAuditObserver отправляет события аудита на удалённый сервер.
//
// Поля:
//   - url: адрес удалённого сервера
//   - client: HTTP-клиент для отправки запросов
type HTTPAuditObserver struct {
	url    string
	client *http.Client
}

// NewHTTPAuditObserver создает новый экземпляр HTTPAuditObserver.
//
// url — адрес удалённого сервера.
//
// Возвращает указатель на HTTPAuditObserver.
func NewHTTPAuditObserver(url string) *HTTPAuditObserver {
	return &HTTPAuditObserver{
		url:    url,
		client: &http.Client{},
	}
}

// OnAuditEvent обрабатывает событие аудита, отправляя его на удалённый сервер.
//
// event — событие аудита для отправки.
//
// Возвращает ошибку при неудаче отправки.
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

// AuditManager управляет списком наблюдателей аудита и уведомляет их о событиях.
//
// Поля:
//   - observers: список наблюдателей (AuditObserver)
//   - mu: RW-мьютекс для синхронизации доступа к списку наблюдателей
type AuditManager struct {
	observers []models.AuditObserver
	mu        sync.RWMutex
}

// NewAuditManager создает новый экземпляр AuditManager.
//
// Возвращает указатель на AuditManager.
func NewAuditManager() *AuditManager {
	return &AuditManager{
		observers: make([]models.AuditObserver, 0),
	}
}

// Attach добавляет наблюдателя к списку.
//
// observer — наблюдатель, реализующий интерфейс AuditObserver.
func (a *AuditManager) Attach(observer models.AuditObserver) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.observers = append(a.observers, observer)
}

// Detach удаляет наблюдателя из списка.
//
// observer — наблюдатель для удаления.
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

// Notify уведомляет всех подключённых наблюдателей о событии.
//
// event — событие аудита для рассылки.
func (a *AuditManager) Notify(event models.AuditEvent) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, observer := range a.observers {
		if err := observer.OnAuditEvent(event); err != nil {
			log.Printf("Audit observer error: %v", err)
		}
	}
}

// HasObservers проверяет, есть ли подключённые наблюдатели.
//
// Возвращает true, если список наблюдателей не пуст.
func (a *AuditManager) HasObservers() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.observers) > 0
}
