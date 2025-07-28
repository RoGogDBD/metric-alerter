package config

import (
	"flag"
	"strconv"
	"strings"
)

type NetAddress struct {
	Host string
	Port int
}

func (a NetAddress) String() string {
	return a.Host + ":" + strconv.Itoa(a.Port)
}

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

func ParseAddressFlag() *NetAddress {
	addr := &NetAddress{Host: "localhost", Port: 8080}
	flag.Var(addr, "a", "Net address host:port")
	return addr
}