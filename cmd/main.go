package main

import (
	"log"

	libuv "github.com/xujintao/go-libuv"
)

func onWrite(conn libuv.Conn) {
	log.Println("write finish")
	// conn.Close() // 不主动关闭连接
}

func onRead(conn libuv.Conn, buf []byte, n int) {
	// 业务要分包的话，需要继续读

	// 业务逻辑
	if n == 0 { // 客户端关闭连接
		log.Println("client close")
		conn.Close()
		return
	}
	log.Println("recv:", string(buf[:n]))

	// 写
	data := "HTTP/1.1 200 OK\r\nServer: nginx/1.15.5\r\n\r\n"
	if err := conn.Write(onWrite, []byte(data)); err != nil {
		log.Println(err)
	}
}

func onAccept(conn libuv.Conn) {
	log.Printf("accept new conn: %s\n", conn.GetAddr())
	buf := make([]byte, 32)

	// 读
	if err := conn.Read(onRead, buf); err != nil {
		log.Println(err)
		conn.Close()
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	libuv.Start(":8080", onAccept)
	libuv.Wait()
}
