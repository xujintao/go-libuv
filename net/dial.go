package net

import (
	"context"
	"net"
	"syscall"
)

func Listen(network, address string) (Listener, error) {

	addr := syscall.SockaddrInet4{Port: 8080}
	copy(addr.Addr[:], net.ParseIP("0.0.0.0").To4())
	return listenTCP(context.Background(), &addr)
	// 或者listenUnix
}

func ListenPacket(newwork, address string) (PacketConn, error) {
	return nil, nil
}
