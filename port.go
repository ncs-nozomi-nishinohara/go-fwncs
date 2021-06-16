package fwncs

import (
	"fmt"
	"net"
)

func findPort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return -1, err
	}
	addr := l.Addr().(*net.TCPAddr)
	if err := l.Close(); err != nil {
		return -1, err
	}
	return addr.Port, nil
}

func getListen(port int) (net.Listener, error) {
	addr := fmt.Sprintf(":%d", port)
	if port == 0 {
		port, err := findPort()
		if err != nil {
			return nil, err
		}
		addr = fmt.Sprintf(":%d", port)
	}
	return net.Listen("tcp", addr)
}
