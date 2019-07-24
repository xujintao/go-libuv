package net

import (
	"errors"
	"fmt"
	"syscall"
)

type Listener interface {
	Accept(func(Conn)) error
	Close() error
	Addr() syscall.Sockaddr
}

type Conn interface {
	Read([]byte, func([]byte, int)) error
	Write([]byte, func([]byte, int)) error
	Close() error
	LocalAddr() syscall.Sockaddr
	RemoteAddr() syscall.Sockaddr
}

type conn struct {
	fd *netFD
}

// Implementation of the Conn interface.

// Read implements the Conn Read method.
func (c *conn) Read(b []byte, handle func(p []byte, n int)) error {
	return c.fd.Read(b, func(p []byte, n int) {
		handle(p, n)
	})
}

// Write implements the Conn Write method.
func (c *conn) Write(b []byte, handle func(p []byte, n int)) error {
	return c.fd.Write(b, func(p []byte, n int) {
		handle(p, n)
	})
}

// Close closes the connection.
func (c *conn) Close() error {
	return c.fd.Close()
}

// LocalAddr returns the local network address.
// The Addr returned is shared by all invocations of LocalAddr, so
// do not modify it.
func (c *conn) LocalAddr() syscall.Sockaddr {
	return c.fd.lsa
}

// RemoteAddr returns the remote network address.
// The Addr returned is shared by all invocations of RemoteAddr, so
// do not modify it.
func (c *conn) RemoteAddr() syscall.Sockaddr {
	return c.fd.rsa
}

type PacketConn interface{}

func Errorf(format string, e ...interface{}) error {
	errstr := fmt.Sprintf(format, e...)
	return errors.New(errstr)
}
