package net

import (
	"context"
	"errors"
	"os"
	"syscall"
)

// syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, 0)
func socket(ctx context.Context, net string, family, sotype, proto int, lsa, rsa syscall.Sockaddr) (fd *netFD, err error) {
	s, err := syscall.Socket(family, sotype|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}

	fd, _ = newFD(s, family, sotype, net)

	if lsa != nil && rsa == nil {
		switch sotype {
		case syscall.SOCK_STREAM, syscall.SOCK_SEQPACKET:
			if err := fd.listenStream(lsa, 128); err != nil {
				fd.Close()
				return nil, err
			}
			return fd, nil
		case syscall.SOCK_DGRAM:
			if err := fd.listenDatagram(lsa); err != nil {
				fd.Close()
				return nil, err
			}
			return fd, nil
		default:
			return nil, errors.New("invalid socket type")
		}
	}

	// fd.dial(ctx)
	return
}

func (fd *netFD) listenStream(lsa syscall.Sockaddr, backlog int) error {
	// 只有listenfd需要设置
	if err := syscall.SetsockoptInt(fd.pd.Sysfd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return os.NewSyscallError("setsockopt", err)
	}

	if err := syscall.Bind(fd.pd.Sysfd, lsa); err != nil {
		return os.NewSyscallError("bind", err)
	}

	if err := syscall.Listen(fd.pd.Sysfd, backlog); err != nil {
		return os.NewSyscallError("listen", err)
	}

	if err := fd.init(); err != nil {
		return err
	}
	fd.lsa = lsa
	return nil
}

func (fd *netFD) listenDatagram(lsa syscall.Sockaddr) error {
	if err := syscall.Bind(fd.pd.Sysfd, lsa); err != nil {
		return os.NewSyscallError("bind", err)
	}
	if err := fd.init(); err != nil {
		return err
	}
	fd.lsa = lsa
	return nil
}
