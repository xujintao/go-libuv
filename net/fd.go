package net

import (
	"syscall"

	poll "github.com/xujintao/go-libuv/poll"
)

type netFD struct {
	pd          poll.PollDesc
	family      int
	sotype      int
	isCOnnected bool
	net         string
	lsa         syscall.Sockaddr
	rsa         syscall.Sockaddr
}

func newFD(sysfd, family, sotype int, net string) (*netFD, error) {
	ret := &netFD{
		pd: poll.PollDesc{
			Sysfd: sysfd,
		},
		family: family,
		sotype: sotype,
		net:    net,
	}
	return ret, nil
}

func (fd *netFD) init() error {
	return fd.pd.Init()
}

func (fd *netFD) Close() error {
	return fd.pd.Close()
}

func (fd *netFD) accept(handle func(*netFD)) error {
	return fd.pd.Accept(func(s int, sa syscall.Sockaddr) {
		netfd, _ := newFD(s, fd.family, fd.sotype, fd.net)
		if err := netfd.init(); err != nil {
			fd.Close() // 连接fd注册失败却关闭监听fd?
		}
		netfd.lsa, _ = syscall.Getsockname(netfd.pd.Sysfd)
		netfd.rsa = sa
		handle(netfd)
	})
}

func (fd *netFD) Read(buf []byte, handle func([]byte, int)) error {
	return fd.pd.Read(buf, func(p []byte, n int) {
		handle(p, n)
	})
}

func (fd *netFD) Write(buf []byte, handle func([]byte, int)) error {
	return fd.pd.Write(buf, func(p []byte, n int) {
		handle(p, n)
	})
}
