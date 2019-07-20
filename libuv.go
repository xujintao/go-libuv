package libuv

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"unsafe"
)

type Conn interface {
	Read(func(Conn, []byte, int), []byte) error
	Write(func(Conn), []byte) error
	Close()
	GetAddr() string
}

type pollDesc struct {
	listenfd   int
	islistener bool
	cbAccept   func(Conn)
	fd         int
	cbRead     func(Conn, []byte, int)
	bufr       []byte
	cbWrite    func(Conn)
	bufw       []byte
	sa         syscall.Sockaddr
}

func (pd *pollDesc) read() error {
	fd := pd.fd
	buf := pd.bufr
	cbRead := pd.cbRead

	// 循环读是担心待接收的数据量太大，读到EAGAIN才算完事
	done := false
	sum := 0
	for {
		n, err := syscall.Read(fd, buf[sum:])
		if n == -1 {
			switch err {
			case syscall.EAGAIN:
				if sum == 0 { // fd还没就绪
					log.Println("fd not ready for reading")
					pd.bufr = buf
					return nil
				}
				done = true
			default:
				log.Println(err)
				return err
			}
		}

		if done || (n == 0 && sum == 0) {
			break
		}

		sum += n
		// 我这边扩容可以解决ET丢数据问题，但并没有解决粘包问题
		// golang那边的协程异步io模型使用ring buffer解决了粘包问题，但没看到它去扩容，它存在丢数据问题
		// 可以这样设计，对ring buffer进行扩容(修改r和w的位置)来解决这两个问题
		if sum == len(buf) {
			buf = append(buf, buf...)
		}
	}

	pd.bufr = buf
	cbRead(Conn(pd), buf, sum)
	return nil
}

func (pd *pollDesc) Read(cbRead func(Conn, []byte, int), buf []byte) error {
	if len(buf) == 0 {
		return errors.New("empty buf")
	}
	pd.cbRead = cbRead
	pd.bufr = buf
	return pd.read()
}

func (pd *pollDesc) write() error {
	fd := pd.fd
	buf := pd.bufw
	cbWrite := pd.cbWrite

	// 循环写的原因担心待发送的数据量太大，先触发EAGAIN，再分批发送
	for {
		n, err := syscall.Write(fd, buf)
		if n == -1 {
			switch err {
			case syscall.EAGAIN:
				log.Println("fd not ready for writing")
				pd.bufw = buf
				return nil
			default:
				log.Println(err)
				return err
			}
		}

		log.Printf("write bytes %d\n", n)
		buf = buf[n:] // 切
		if 0 == len(buf) {
			break
		}
	}

	cbWrite(Conn(pd))
	pd.bufw = buf[:0]
	return nil
}

func (pd *pollDesc) Write(cbWrite func(Conn), buf []byte) error {
	if len(buf) == 0 {
		return errors.New("nothing to write")
	}
	pd.cbWrite = cbWrite

	// 直接赋值会有问题，比如上一个事务的数据还没写完而本次事务又有数据需要写，会覆盖
	pd.bufw = append(pd.bufw, buf...)
	return pd.write()
}

func (pd *pollDesc) Close() {
	// 删除io监听
	var event syscall.EpollEvent
	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, pd.fd, &event); err != nil {
		log.Println(err)
		return
	}

	// 主动关闭连接
	if err := syscall.Close(pd.fd); err != nil {
		log.Println(err)
		return
	}
}

func (pd *pollDesc) GetAddr() string {

	var addr string
	switch sa := pd.sa.(type) {
	case *syscall.SockaddrInet4:
		ip := net.IPv4(sa.Addr[0], sa.Addr[1], sa.Addr[2], sa.Addr[3]).String()
		port := sa.Port
		addr = fmt.Sprintf("%s:%d", ip, port)
	case *syscall.SockaddrInet6:
	case *syscall.SockaddrUnix:
	default:
		return "invalid addr"
	}
	return addr
}

var (
	epfd int = -1
)

func init() {
	var err error
	epfd, err = syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

}

func Wait() {
	var events [128]syscall.EpollEvent
	for {
		// msec参数runtime那边是0，非阻塞。这里设置为-1，阻塞
		n, err := syscall.EpollWait(epfd, events[:], -1)
		if err != nil {
			log.Println(err)
			break
		}

		for i := 0; i < n; i++ {
			e := events[i]
			if e.Events == 0 {
				log.Println("wait continue")
				continue
			}

			if e.Events&(syscall.EPOLLERR|syscall.EPOLLHUP) != 0 {
				log.Println(e.Events)
				pd := *(**pollDesc)(unsafe.Pointer(&e.Fd))
				pd.Close()
				continue
			}

			switch {
			case e.Events&syscall.EPOLLIN != 0:
				pd := *(**pollDesc)(unsafe.Pointer(&e.Fd))
				if pd.islistener {
					accept(epfd, e)
					break
				}
				read(epfd, e)
			case e.Events&syscall.EPOLLOUT != 0:
				write(epfd, e)
			default:
				fmt.Printf("poll continue, event(%v)", e)
			}
		}
	}
}

// Start 启动
func Start(address string, onAccept func(Conn)) {
	listenfd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// 只有listenfd需要设置
	if err := syscall.SetsockoptInt(listenfd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	addr := syscall.SockaddrInet4{Port: 8080}
	copy(addr.Addr[:], net.ParseIP("0.0.0.0").To4())
	if err := syscall.Bind(listenfd, &addr); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if err := syscall.Listen(listenfd, 128); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	lsa, _ := syscall.Getsockname(listenfd)
	log.Println(lsa)

	pd := pollDesc{
		listenfd:   listenfd,
		islistener: true,
		cbAccept:   onAccept,
	}
	var event syscall.EpollEvent
	*(**pollDesc)(unsafe.Pointer(&event.Fd)) = &pd
	event.Events = syscall.EPOLLIN | syscall.EPOLLOUT | syscall.EPOLLRDHUP | (-syscall.EPOLLET)
	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, listenfd, &event); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func accept(epfd int, e syscall.EpollEvent) {
	lpd := *(**pollDesc)(unsafe.Pointer(&e.Fd))
	listenfd := lpd.listenfd
	for {
		connfd, sa, err := syscall.Accept4(listenfd, syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC)
		if connfd == -1 {
			switch err {
			case syscall.EAGAIN:
				log.Println("we have processed all incoming conn")
			default:
				log.Println(err)
			}
			return
		}

		// v, err := syscall.GetsockoptInt(connfd, syscall.SOL_SOCKET, syscall.SO_SNDBUF)
		// if err != nil {
		// 	return
		// }
		// // 把发送缓存设小，用于测试
		// if err := syscall.SetsockoptInt(connfd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 40); err != nil {
		// 	log.Println(err)
		// 	return
		// }
		// v, err = syscall.GetsockoptInt(connfd, syscall.SOL_SOCKET, syscall.SO_SNDBUF)
		// if err != nil {
		// 	return
		// }
		// _ = v
		// 小不了

		cpd := pollDesc{
			fd: connfd,
			sa: sa,
		}

		var event syscall.EpollEvent
		*(**pollDesc)(unsafe.Pointer(&event.Fd)) = &cpd
		event.Events = syscall.EPOLLIN | syscall.EPOLLOUT | syscall.EPOLLRDHUP | (-syscall.EPOLLET)
		if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, connfd, &event); err != nil {
			log.Println(err)
			os.Exit(1)
		}

		// 回调用户代码
		lpd.cbAccept(Conn(&cpd))
	}
}

func read(epfd int, e syscall.EpollEvent) error {
	pd := *(**pollDesc)(unsafe.Pointer(&e.Fd))

	// 这里有问题，如果连接来了，用户协议是先写那么就没有配读缓存及读回调
	// 然后就会丢失客户端的关闭连接
	if len(pd.bufr) == 0 {
		log.Println("empty buf")
		return nil
	}
	return pd.read()
}

func write(epfd int, e syscall.EpollEvent) error {
	pd := *(**pollDesc)(unsafe.Pointer(&e.Fd))
	if len(pd.bufw) == 0 {
		log.Println("nothing to write")
		return nil
	}
	return pd.write()
}
