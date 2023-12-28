package lib

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

func CopyDuplex(c0, c1 net.Conn) {
	defer c0.Close()
	defer c1.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	f := func(c0, c1 net.Conn) {
		defer wg.Done()
		io.Copy(c0, c1)
		// Signal peer that no more data is coming.
		if tcpConn, ok := c0.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}
	go f(c0, c1)
	go f(c1, c0)

	wg.Wait()
}

type Pipe struct {
	Fds   [2]int
	Files [2]*os.File
}

func (p *Pipe) Unixpair() (conn [2]net.Conn, err error) {
	if p.Fds, err = unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0); err != nil {
		err = fmt.Errorf("UnixListener: socketpair: %v", err)
		return
	}

	p.Files[0] = os.NewFile(uintptr(p.Fds[0]), "fd0")
	p.Files[1] = os.NewFile(uintptr(p.Fds[1]), "fd1")

	if conn[0], err = net.FileConn(p.Files[0]); err != nil {
		err = fmt.Errorf("UnixListener: fileconn 0: %v", err)
		return
	}

	if conn[1], err = net.FileConn(p.Files[1]); err != nil {
		err = fmt.Errorf("UnixListener: fileconn 1: %v", err)
	}

	return
}

func ConnFd(conn syscall.Conn) (connFd int, err error) {
	var rawConn syscall.RawConn
	rawConn, err = conn.SyscallConn()
	if err != nil {
		return
	}

	err = rawConn.Control(func(fd uintptr) {
		connFd = int(fd)
	})
	return
}
