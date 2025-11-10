package models

// AuditEvent представляет событие аудита
type AuditEvent struct {
	Timestamp int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// AuditObserver интерфейс наблюдателя для аудита
type AuditObserver interface {
	OnAuditEvent(event AuditEvent) error
}

// AuditSubject интерфейс субъекта, генерирующего события аудита
type AuditSubject interface {
	Attach(observer AuditObserver)
	Detach(observer AuditObserver)
	Notify(event AuditEvent)
}
