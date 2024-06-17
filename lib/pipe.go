package lib

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

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

func UnixPipe() (conn [2]net.Conn, err error) {
	var fds [2]int
	var files [2]*os.File
	if fds, err = unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0); err != nil {
		err = fmt.Errorf("unixpipe: unix.socketpair: %v", err)
		return
	}

	files[0] = os.NewFile(uintptr(fds[0]), "fd0")
	files[1] = os.NewFile(uintptr(fds[1]), "fd1")

	defer files[0].Close()
	defer files[1].Close()

	if conn[0], err = net.FileConn(files[0]); err != nil {
		err = fmt.Errorf("unixpipe: fileconn 0: %v", err)
		return
	}

	if conn[1], err = net.FileConn(files[1]); err != nil {
		err = fmt.Errorf("unixpipe: fileconn 1: %v", err)
	}

	return
}
