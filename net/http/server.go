package http

import (
	"fmt"
	"log"

	"github.com/xujintao/go-libuv/net"
	"github.com/xujintao/go-libuv/poll"
)

type conn struct {
	server *Server
	rwc    net.Conn
	bufr   []byte
	bufw   []byte
}

func (c *conn) serve() error {
	err := c.rwc.Read(c.bufr, func(p []byte, n int) {
		// 打印调用栈
		log.Print("read finish:", poll.CallerFuncNamesf(1, 50))

		// 被动关闭连接
		if n == 0 {
			log.Printf("%v close", c.rwc.RemoteAddr())
			c.rwc.Close()
			return
		}

		// 业务逻辑
		log.Print(string(p[:n]))

		// 写
		body := "libuv\r\n"
		format := "HTTP/1.1 200 OK\r\nServer: nginx/1.15.5\r\nContent-Length: %d\r\n\r\n%s"
		body += poll.CallerFuncNamesf(1, 50)
		res := fmt.Sprintf(format, len(body), string(body))
		err := c.rwc.Write([]byte(res), func(p []byte, n int) {
			// 打印调用栈
			log.Print("write finish:", poll.CallerFuncNamesf(1, 50))
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
		// 打印调用栈
		log.Print("accept finish:", poll.CallerFuncNamesf(1, 50))
		c := srv.newConn(rw)
		c.serve()
	})

}

func ListenAndServe(addr string) error {
	server := &Server{Addr: addr}
	return server.ListenAndServe()
}
