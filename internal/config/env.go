package config

import (
	"fmt"
	"os"
	"strconv"
)

// AddrSetter определяет интерфейс для установки адреса из строки.
type AddrSetter interface {
	Set(string) error
}

// EnvServer устанавливает адрес сервера из переменной окружения.
//
// Если переменная окружения с именем envKey присутствует, функция вызывает метод Set интерфейса AddrSetter
// с её значением. В случае ошибки возвращает ошибку с описанием.
//
// addr   — объект, реализующий интерфейс AddrSetter.
// envKey — имя переменной окружения.
//
// Возвращает ошибку, если значение некорректно, иначе nil.
func EnvServer(addr AddrSetter, envKey string) error {
	if envVal, ok := os.LookupEnv(envKey); ok {
		if err := addr.Set(envVal); err != nil {
			return fmt.Errorf("invalid %s: %w", envKey, err)
		}
	}
	return nil
}

// EnvInt возвращает значение переменной окружения как int.
//
// key — имя переменной окружения.
//
// Если переменная не задана или пуста, возвращает 0 и nil.
// Если значение не может быть преобразовано в int, возвращает ошибку.
func EnvInt(key string) (int, error) {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return 0, nil
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return i, nil
}

// EnvString возвращает значение переменной окружения как строку.
//
// key — имя переменной окружения.
//
// Если переменная не задана или пуста, возвращает пустую строку.
func EnvString(key string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return ""
}
