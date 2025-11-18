package repository

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	models "github.com/RoGogDBD/metric-alerter/internal/model"
	"github.com/stretchr/testify/require"
)

// TestFileAuditObserver_OnAuditEvent_TableDriven выполняет табличные тесты для метода OnAuditEvent структуры FileAuditObserver.
//
// Проверяет, что события аудита корректно записываются в файл, включая создание вложенных директорий.
// Для каждого теста сравнивает содержимое файла с ожидаемой строкой.
func TestFileAuditObserver_OnAuditEvent_TableDriven(t *testing.T) {
	tmpDir := t.TempDir()
	tests := []struct {
		name     string            // Название теста
		filePath string            // Путь к файлу аудита
		event    models.AuditEvent // Событие аудита для записи
		wantLine string            // Ожидаемая строка в файле
		wantErr  bool              // Ожидается ли ошибка
	}{
		{
			name:     "write simple event",
			filePath: filepath.Join(tmpDir, "audit.log"),
			event:    models.AuditEvent{Timestamp: time.Now().Unix(), Metrics: []string{"create"}, IPAddress: "127.0.0.1"},
			wantLine: `"metrics":["create"]`,
			wantErr:  false,
		},
		{
			name:     "create nested dir",
			filePath: filepath.Join(tmpDir, "nested", "audit.log"),
			event:    models.AuditEvent{Timestamp: time.Now().Unix(), Metrics: []string{"delete"}},
			wantLine: `"metrics":["delete"]`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			obs := NewFileAuditObserver(tt.filePath)
			err := obs.OnAuditEvent(tt.event)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			f, err := os.Open(tt.filePath)
			require.NoError(t, err)
			defer func() { _ = f.Close() }()
			r := bufio.NewReader(f)
			line, err := r.ReadString('\n')
			require.NoError(t, err)
			require.Contains(t, line, tt.wantLine)
		})
	}
}

// TestHTTPAuditObserver_OnAuditEvent_TableDriven выполняет табличные тесты для метода OnAuditEvent структуры HTTPAuditObserver.
//
// Проверяет, что события аудита корректно отправляются на HTTP-сервер и обрабатываются различные коды ответа.
// Для успешных случаев проверяет, что отправленные данные содержат нужные поля.
func TestHTTPAuditObserver_OnAuditEvent_TableDriven(t *testing.T) {
	tests := []struct {
		name        string // Название теста
		respondCode int    // Код ответа сервера
		wantErr     bool   // Ожидается ли ошибка
	}{
		{"ok 200", http.StatusOK, false},
		{"created 201", http.StatusCreated, false},
		{"server error 500", http.StatusInternalServerError, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var received bytes.Buffer
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				received.Write(b)
				w.WriteHeader(tt.respondCode)
			}))
			defer srv.Close()

			obs := NewHTTPAuditObserver(srv.URL)
			e := models.AuditEvent{Timestamp: time.Now().Unix(), Metrics: []string{"upd"}, IPAddress: "127.0.0.1"}
			err := obs.OnAuditEvent(e)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotZero(t, received.Len())
				// Проверяет, что отправленный JSON содержит поле metrics и значение upd
				require.Contains(t, received.String(), `"metrics"`)
				require.Contains(t, received.String(), "upd")
			}
		})
	}
}

// TestAuditManager_TableDriven выполняет табличные тесты для структуры AuditManager.
//
// Проверяет корректность добавления и удаления наблюдателей, а также рассылку событий.
// Для каждого теста проверяет, что событие записано в файл, если ожидается, и что список наблюдателей корректно очищается.
func TestAuditManager_TableDriven(t *testing.T) {
	mgr := NewAuditManager()

	fpath := filepath.Join(t.TempDir(), "am.log")
	fileObs := NewFileAuditObserver(fpath)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	httpObs := NewHTTPAuditObserver(srv.URL)

	tests := []struct {
		name     string                 // Название теста
		attach   []models.AuditObserver // Список наблюдателей для подключения
		event    models.AuditEvent      // Событие аудита для рассылки
		wantFile bool                   // Ожидать ли запись в файл
	}{
		{"single file observer", []models.AuditObserver{fileObs}, models.AuditEvent{Timestamp: time.Now().Unix(), Metrics: []string{"t1"}}, true},
		{"file + http", []models.AuditObserver{fileObs, httpObs}, models.AuditEvent{Timestamp: time.Now().Unix(), Metrics: []string{"t2"}}, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			for _, o := range tt.attach {
				mgr.Attach(o)
			}
			require.True(t, mgr.HasObservers())
			mgr.Notify(tt.event)

			if tt.wantFile {
				f, err := os.Open(fpath)
				require.NoError(t, err)
				defer func() { _ = f.Close() }()
				s := bufio.NewScanner(f)
				found := false
				for s.Scan() {
					if bytes.Contains(s.Bytes(), []byte(tt.event.Metrics[0])) {
						found = true
						break
					}
				}
				require.True(t, found, "expected to find event Metrics in file")
			}

			for _, o := range tt.attach {
				mgr.Detach(o)
			}
			require.False(t, mgr.HasObservers())
		})
	}
}
