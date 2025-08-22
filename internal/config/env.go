package config

import (
	"fmt"
	"os"
	"strconv"
)

type AddrSetter interface {
	Set(string) error
}

func EnvServer(addr AddrSetter, envKey string) error {
	if envVal, ok := os.LookupEnv(envKey); ok {
		if err := addr.Set(envVal); err != nil {
			return fmt.Errorf("invalid %s: %w", envKey, err)
		}
	}
	return nil
}

func EnvInt(key string) (int, error) {
	val, ok := os.LookupEnv(key)
	if !ok {
		return 0, nil
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return i, nil
}
