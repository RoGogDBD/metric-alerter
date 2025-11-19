package models

// Counter — константа, обозначающая тип метрики "счётчик".
// Счётчики увеличиваются на указанное значение (delta).
const Counter = "counter"

// Gauge — константа, обозначающая тип метрики "датчик".
// Датчики устанавливаются в указанное значение (value).
const Gauge = "gauge"

// Metrics представляет метрику для сериализации в JSON.
//
// Структура использует плоскую модель без вложенности.
// Delta и Value объявлены через указатели для различения
// значения "0" от отсутствующего значения при сериализации.
//
// Поля:
//   - ID: уникальный идентификатор метрики
//   - MType: тип метрики (Counter или Gauge)
//   - Delta: приращение для счётчика (используется для Counter)
//   - Value: значение для датчика (используется для Gauge)
//   - Hash: HMAC-SHA256 подпись метрики (опционально)
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	Hash  string   `json:"hash,omitempty"`
}
