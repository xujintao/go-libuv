package net

import (
	"context"
	"syscall"
)

type TCPConn struct {
	conn
}

func newTCPConn(fd *netFD) *TCPConn {
	c := &TCPConn{conn{fd: fd}}
	syscall.SetsockoptInt(fd.pd.Sysfd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
	return c
}

type TCPListener struct {
	fd *netFD
}

// Accept 和 Accpept(func(*Conn))有什么区别？
func (l *TCPListener) Accept(handle func(Conn)) error {
	return l.fd.accept(func(fd *netFD) {
		c := newTCPConn(fd)
		handle(c)
	})
}

func (l *TCPListener) Close() error {
	return l.fd.Close()
}

func (l *TCPListener) Addr() syscall.Sockaddr {
	return l.fd.lsa
}

func listenTCP(ctx context.Context, lsa syscall.Sockaddr) (*TCPListener, error) {
	fd, err := socket(ctx, "tcp", syscall.AF_INET, syscall.SOCK_STREAM, 0, lsa, nil)
	if err != nil {
		return nil, err
	}

	tcpListener := &TCPListener{fd: fd}
	return tcpListener, nil
}
