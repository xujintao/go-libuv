package net

import (
	"context"
	"net"
	"strconv"
	"strings"
	"syscall"
)

// SplitHostPort splits a network address of the form "host:port",
// into host and port
func SplitHostPort(hostport string) (host string, port int, err error) {
	const (
		missingPort   = "missing port in address"
		tooManyColons = "too many colons in address"
	)
	i := strings.LastIndexByte(hostport, ':')
	if i < 0 {
		return "", 0, Errorf("%s %s", hostport, missingPort)
	}
	host = hostport[:i]
	if strings.IndexByte(host, ':') >= 0 {
		return "", 0, Errorf("%s %s", hostport, tooManyColons)
	}
	port, err = strconv.Atoi(hostport[i+1:])
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

// Listen announces on the local network address.
// network: must be "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
// address host: can be empty or host name(not recommended)
// address port: can be empty or 0
func Listen(network, address string) (Listener, error) {
	// 地址解析
	host, port, err := SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	// 标准库根据network参数将address串解析成一个接口，可以是tcp地址也可以是unix地址
	addr := syscall.SockaddrInet4{Port: port}
	copy(addr.Addr[:], net.ParseIP(host).To4())
	return listenTCP(context.Background(), &addr)
	// 或者listenUnix
}

func ListenPacket(newwork, address string) (PacketConn, error) {
	return nil, nil
}
