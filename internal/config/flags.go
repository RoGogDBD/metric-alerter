package config

import (
	"flag"
	"strconv"
	"strings"
)

// NetAddress представляет сетевой адрес с хостом и портом.
type NetAddress struct {
	Host string // Имя хоста
	Port int    // Порт
}

// String возвращает строковое представление сетевого адреса в формате host:port.
func (a NetAddress) String() string {
	return a.Host + ":" + strconv.Itoa(a.Port)
}

// Set разбирает строку вида host:port и устанавливает значения Host и Port.
//
// Если порт не указан, по умолчанию используется 8080.
// Возвращает ошибку, если порт не удаётся преобразовать в число.
func (a *NetAddress) Set(s string) error {
	hp := strings.Split(s, ":")
	a.Host = hp[0]
	if len(hp) == 2 {
		port, err := strconv.Atoi(hp[1])
		if err != nil {
			return err
		}
		a.Port = port
	} else {
		a.Port = 8080
	}
	return nil
}

// ParseAddressFlag регистрирует флаг командной строки -a для указания сетевого адреса.
//
// Возвращает указатель на NetAddress с дефолтными значениями (localhost:8080).
func ParseAddressFlag() *NetAddress {
	addr := &NetAddress{Host: "localhost", Port: 8080}
	flag.Var(addr, "a", "Net address host:port")
	return addr
}
