package models

// AuditEvent представляет событие аудита.
//
// Поля:
//   - Timestamp: временная метка события (Unix-время, int64)
//   - Metrics: список имён метрик, связанных с событием
//   - IPAddress: IP-адрес клиента, вызвавшего событие
type AuditEvent struct {
	Timestamp int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// AuditObserver интерфейс наблюдателя для аудита.
//
// Любой тип, реализующий этот интерфейс, может получать уведомления о событиях аудита.
type AuditObserver interface {
	// OnAuditEvent вызывается при возникновении события аудита.
	// Возвращает ошибку в случае неудачи обработки события.
	OnAuditEvent(event AuditEvent) error
}

// AuditSubject интерфейс субъекта, генерирующего события аудита.
//
// Позволяет подписывать и отписывать наблюдателей, а также рассылать им уведомления о событиях.
type AuditSubject interface {
	// Attach добавляет наблюдателя для получения событий аудита.
	Attach(observer AuditObserver)
	// Detach удаляет наблюдателя.
	Detach(observer AuditObserver)
	// Notify рассылает событие всем подписанным наблюдателям.
	Notify(event AuditEvent)
}
