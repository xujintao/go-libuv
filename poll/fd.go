package poll

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"unsafe"
)

type PollDesc struct {
	Sysfd    int
	bufr     []byte
	bufw     []byte
	OnAccept func(int, syscall.Sockaddr)
	OnRead   func([]byte, int)
	OnWrite  func([]byte, int)
}

func (pd *PollDesc) Init() error {
	var event syscall.EpollEvent
	*(**PollDesc)(unsafe.Pointer(&event.Fd)) = pd
	event.Events = syscall.EPOLLIN | syscall.EPOLLOUT | syscall.EPOLLRDHUP | (-syscall.EPOLLET)
	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, pd.Sysfd, &event); err != nil {
		uvlog.Println(err)
		return err
	}
	return nil
}

func (pd *PollDesc) read() error {
	fd := pd.Sysfd
	buf := pd.bufr

	// 循环读是担心待接收的数据量太大，读到EAGAIN才算完事
	done := false
	sum := 0
	for {
		n, err := syscall.Read(fd, buf[sum:])
		if n == -1 {
			switch err {
			case syscall.EAGAIN:
				if sum == 0 { // fd还没就绪
					uvlog.Print("fd not ready for reading")
					pd.bufr = buf
					return nil
				}
				done = true
			default:
				uvlog.Print(err)
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
	pd.OnRead(pd.bufr, sum)
	return nil
}

func (pd *PollDesc) Read(buf []byte, handle func([]byte, int)) error {
	if len(buf) == 0 {
		return errors.New("empty buf")
	}
	pd.bufr = buf
	pd.OnRead = handle
	return pd.read()
}

func (pd *PollDesc) write() error {
	fd := pd.Sysfd
	buf := pd.bufw

	// 循环写的原因担心待发送的数据量太大，先触发EAGAIN，再分批发送
	for {
		n, err := syscall.Write(fd, buf)
		if n == -1 {
			switch err {
			case syscall.EAGAIN:
				uvlog.Print("fd not ready for writing")
				pd.bufw = buf
				return nil
			default:
				uvlog.Println(err)
				return err
			}
		}

		uvlog.Printf("write bytes %d", n)
		buf = buf[n:] // 切
		if 0 == len(buf) {
			break
		}
	}

	// pd.Handler.OnWrite(sum)
	pd.OnWrite(pd.bufw, 50)
	pd.bufw = buf[:0]
	return nil
}

func (pd *PollDesc) Write(buf []byte, handle func([]byte, int)) error {
	if len(buf) == 0 {
		return errors.New("nothing to write")
	}

	// 直接赋值会有问题，比如上一个事务的数据还没写完而本次事务又有数据需要写，会覆盖
	pd.bufw = append(pd.bufw, buf...)
	pd.OnWrite = handle
	return pd.write()
}

func (pd *PollDesc) Close() error {
	// 删除io监听
	var event syscall.EpollEvent
	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, pd.Sysfd, &event); err != nil {
		uvlog.Print(err)
		return err
	}

	// 主动关闭连接
	if err := syscall.Close(pd.Sysfd); err != nil {
		uvlog.Print(err)
		return err
	}
	return nil
}

func (pd *PollDesc) accept() error {
	fd := pd.Sysfd

	done := false
	cnt := 0
	for {
		connfd, sa, err := syscall.Accept4(fd, syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC)
		if connfd == -1 {
			switch err {
			case syscall.EAGAIN:
				if cnt == 0 {
					uvlog.Print("connect not yet")
					return nil
				}
				done = true

			default:
				uvlog.Print(err)
				return err
			}
		}

		if done {
			uvlog.Printf("we have processed %d incoming conn", cnt)
			break
		}

		cnt++

		// 回调用户代码
		pd.OnAccept(connfd, sa)
	}
	return nil
}

func (pd *PollDesc) Accept(handle func(int, syscall.Sockaddr)) error {
	// 调用的时候，也是回调函数压栈的过程
	// 保存了回调函数栈头，出栈的过程正好也是回调流的过程，相当完美
	pd.OnAccept = handle
	return pd.accept()
}

func getAddr(sa syscall.Sockaddr) string {

	var addr string
	switch sa := sa.(type) {
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
	epfd  int = -1
	uvlog     = log.New(os.Stdout, "[UVLOG] ", log.LstdFlags|log.Lshortfile)
)

func init() {
	var err error
	epfd, err = syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		uvlog.Println(err)
		os.Exit(1)
	}
}

func Wait() {
	var events [128]syscall.EpollEvent
	for {
		// msec参数runtime那边是0，非阻塞。这里设置为-1，阻塞
		n, err := syscall.EpollWait(epfd, events[:], -1)
		if err != nil {
			uvlog.Println(err)
			break
		}

		for i := 0; i < n; i++ {
			e := events[i]
			if e.Events == 0 {
				uvlog.Println("wait continue")
				continue
			}

			if e.Events&(syscall.EPOLLERR|syscall.EPOLLHUP) != 0 {
				uvlog.Println(e.Events)
				pd := *(**PollDesc)(unsafe.Pointer(&e.Fd))
				pd.Close()
				continue
			}

			switch {
			case e.Events&syscall.EPOLLIN != 0:
				pd := *(**PollDesc)(unsafe.Pointer(&e.Fd))
				if pd.OnAccept != nil {
					uvlog.Println("accept event active------>")
					accept(epfd, e)
					uvlog.Println("------>accept event deprecated")
					break
				}
				uvlog.Println("read event active------>")
				read(epfd, e)
				uvlog.Println("------>read event deprecated")
			case e.Events&syscall.EPOLLOUT != 0:
				uvlog.Println("write event active------>")
				write(epfd, e)
				uvlog.Println("------>write event deprecated")
			default:
				uvlog.Printf("poll continue, event(%v)", e)
			}
		}
	}
}

// Start 启动
// func Start(address string, handler Handler) {

// 	uvlog.Println("uv start")

// 	// pd.Accept()
// }

func accept(epfd int, e syscall.EpollEvent) error {
	lpd := *(**PollDesc)(unsafe.Pointer(&e.Fd))
	return lpd.accept()
}

func read(epfd int, e syscall.EpollEvent) error {
	pd := *(**PollDesc)(unsafe.Pointer(&e.Fd))

	// 这里有问题，如果连接来了，用户协议是先写那么就没有配读缓存及读回调
	// 然后就会丢失客户端的关闭连接
	if len(pd.bufr) == 0 {
		uvlog.Print("empty buf")
		return nil
	}
	return pd.read()
}

func write(epfd int, e syscall.EpollEvent) error {
	pd := *(**PollDesc)(unsafe.Pointer(&e.Fd))
	if len(pd.bufw) == 0 {
		uvlog.Print("nothing to write")
		return nil
	}
	return pd.write()
}
