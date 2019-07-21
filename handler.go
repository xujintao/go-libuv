package libuv

type Handler interface {
	OnAccept(Conn)
	OnRead(Conn, []byte, int)
	OnWrite(Conn)
}
