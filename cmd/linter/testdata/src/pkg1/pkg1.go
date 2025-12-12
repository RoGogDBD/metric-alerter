package pkg

import (
	"log"
	"os"
)

// panic - детектит.
func FuncWithPanic() {
	panic("ошибка") // want "use of builtin panic is discouraged"
}

// log.Fatal - детектит.
func FuncWithFatal() {
	log.Fatal("вне main.main") // want "call to log.Fatal or os.Exit outside main.main"
}

// os.Exit - детектит.
func FuncWithExit() {
	os.Exit(1) // want "call to log.Fatal or os.Exit outside main.main"
}

// log.Print - всё ГУДчи.
func FuncAllowed() {
	log.Println("ОК")
}
