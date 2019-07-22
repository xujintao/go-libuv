package http

import (
	"log"

	"github.com/xujintao/go-libuv/net"
)

type conn struct {
	server *Server
	rwc    net.Conn
	bufr   []byte
	bufw   []byte
}

func (c *conn) serve() error {
	err := c.rwc.Read(c.bufr, func(p []byte, n int) {
		// 被动关闭连接
		if n == 0 {
			log.Printf("%v close", c.rwc.RemoteAddr())
			c.rwc.Close()
			return
		}

		// 业务逻辑
		log.Print(string(p[:n]))

		// 写
		data := "HTTP/1.1 200 OK\r\nServer: nginx/1.15.5\r\nContent-Length: 7\r\n\r\nlibuv\r\n"
		err := c.rwc.Write([]byte(data), func(p []byte, n int) {
			log.Println("write finish")
		})
		if err != nil {
			log.Print(err)
			return
		}

	})
	if err != nil {
		log.Print(err)
		return err
	}
	return nil
}

type Server struct {
	Addr string
	bufr []byte
	bufw []byte
}

func (srv *Server) newConn(rwc net.Conn) *conn {
	c := &conn{
		server: srv,
		rwc:    rwc,
		bufr:   make([]byte, 32),
		bufw:   make([]byte, 32),
	}
	return c
}

func (srv *Server) ListenAndServe() error {
	l, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	return srv.Serve(l)
}

func (srv *Server) Serve(l net.Listener) error {
	return l.Accept(func(rw net.Conn) {
		c := srv.newConn(rw)
		c.serve()
	})

}

func ListenAndServe(addr string) error {
	server := &Server{Addr: addr}
	return server.ListenAndServe()
}
