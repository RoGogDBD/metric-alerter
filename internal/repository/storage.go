package repository

import (
	"strconv"
	"sync"
)

// Storage определяет интерфейс для работы с хранилищем метрик.
//
// Позволяет устанавливать и получать значения gauge и counter, а также получать все метрики.
type Storage interface {
	// SetGauge устанавливает значение gauge-метрики по имени.
	SetGauge(name string, value float64)
	// AddCounter увеличивает значение counter-метрики по имени на delta.
	AddCounter(name string, delta int64)
	// GetGauge возвращает значение gauge-метрики по имени и флаг наличия.
	GetGauge(name string) (float64, bool)
	// GetCounter возвращает значение counter-метрики по имени и флаг наличия.
	GetCounter(name string) (int64, bool)
	// GetAll возвращает срез всех метрик в виде MetricInfo.
	GetAll() []MetricInfo
}

// MemStorage реализует интерфейс Storage на основе памяти.
//
// Использует map для хранения gauge и counter, защищённых мьютексом.
type MemStorage struct {
	gauge   map[string]float64 // Хранилище gauge-метрик
	counter map[string]int64   // Хранилище counter-метрик
	mu      sync.RWMutex       // Мьютекс для конкурентного доступа
}

// MetricInfo содержит информацию о метрике для сериализации/вывода.
//
// Name — имя метрики.
// Type — тип метрики ("gauge" или "counter").
// Value — строковое представление значения.
type MetricInfo struct {
	Name  string
	Type  string
	Value string
}

// MetricUpdate описывает обновление метрики.
//
// Type — тип метрики.
// Name — имя метрики.
// FloatVal — указатель на значение для gauge.
// IntVal — указатель на значение для counter.
type MetricUpdate struct {
	Type     string
	Name     string
	FloatVal *float64
	IntVal   *int64
}

// NewMemStorage создаёт и возвращает новый экземпляр MemStorage.
//
// Возвращает Storage с пустыми map для gauge и counter.
func NewMemStorage() Storage {
	return &MemStorage{
		gauge:   make(map[string]float64),
		counter: make(map[string]int64),
	}
}

// SetGauge устанавливает значение gauge-метрики по имени.
//
// name — имя метрики.
// value — значение метрики.
func (s *MemStorage) SetGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauge[name] = value
}

// AddCounter увеличивает значение counter-метрики по имени на delta.
//
// name — имя метрики.
// delta — приращение.
func (s *MemStorage) AddCounter(name string, delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter[name] += delta
}

// GetGauge возвращает значение gauge-метрики по имени и флаг наличия.
//
// name — имя метрики.
// Возвращает значение и true, если метрика найдена.
func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.gauge[name]
	return val, ok
}

// GetCounter возвращает значение counter-метрики по имени и флаг наличия.
//
// name — имя метрики.
// Возвращает значение и true, если метрика найдена.
func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.counter[name]
	return val, ok
}

// GetAll возвращает срез всех метрик в виде MetricInfo.
//
// Формирует список из всех gauge и counter метрик с их значениями.
func (s *MemStorage) GetAll() []MetricInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MetricInfo
	for k, v := range s.gauge {
		result = append(result, MetricInfo{
			Name:  k,
			Type:  "gauge",
			Value: strconv.FormatFloat(v, 'f', -1, 64),
		})
	}
	for k, v := range s.counter {
		result = append(result, MetricInfo{
			Name:  k,
			Type:  "counter",
			Value: strconv.FormatInt(v, 10),
		})
	}
	return result
}
