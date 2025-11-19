package config

import (
	"strconv"
	"testing"
)

// TestNetAddress_SetAndString_TableDriven проверяет методы Set и String структуры NetAddress с помощью табличных тестов.
//
// Для каждого тестового случая проверяется корректность разбора строки адреса, установка host и port,
// а также корректность строкового представления адреса. Также тестируется обработка ошибочных входных данных.
func TestNetAddress_SetAndString_TableDriven(t *testing.T) {
	tests := []struct {
		name      string // Название теста
		input     string // Входная строка для метода Set
		exHost    string // Ожидаемый host после Set
		exPort    int    // Ожидаемый port после Set
		expectErr bool   // Ожидается ли ошибка
	}{
		{"host:port", "localhost:9000", "localhost", 9000, false},
		{"only host", "example", "example", 8080, false},
		{"empty string", "", "", 8080, false},
		{"empty host with port", ":9090", "", 9090, false},
		{"bad port", "host:notaport", "", 0, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var a NetAddress
			err := a.Set(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}
			if a.Host != tt.exHost {
				t.Fatalf("host mismatch: expected %q, got %q", tt.exHost, a.Host)
			}
			if a.Port != tt.exPort {
				t.Fatalf("port mismatch: expected %d, got %d", tt.exPort, a.Port)
			}
			expectedStr := a.Host + ":" + func() string {
				if a.Port == 0 {
					return "0"
				}
				return strconv.Itoa(a.Port)
			}()
			if a.String() != expectedStr {
				t.Fatalf("String() mismatch: expected %q, got %q", expectedStr, a.String())
			}
		})
	}
}

// TestParseAddressFlag_Defaults проверяет, что функция ParseAddressFlag возвращает NetAddress с дефолтными значениями.
//
// Проверяется, что host равен "localhost", а port — 8080.
func TestParseAddressFlag_Defaults(t *testing.T) {
	addr := ParseAddressFlag()
	if addr == nil {
		t.Fatal("ParseAddressFlag returned nil")
	}
	if addr.Host != "localhost" {
		t.Fatalf("default host expected %q, got %q", "localhost", addr.Host)
	}
	if addr.Port != 8080 {
		t.Fatalf("default port expected %d, got %d", 8080, addr.Port)
	}
}
