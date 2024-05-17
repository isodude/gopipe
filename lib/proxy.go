package lib

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

type Proxy interface {
	Proxy(*Listen, *Client) error
}

type CloseWriter struct {
	net.Conn
}

func (c *CloseWriter) NetConn() net.Conn {
	return c.Conn
}

func (c *CloseWriter) Close() error {
	switch conn := c.Conn.(type) {
	case *net.TCPConn:
		if err := conn.CloseWrite(); err != nil {
			err = fmt.Errorf("(%s->%s): CloseWrite failed: %v", c.Conn.LocalAddr(), c.Conn.RemoteAddr(), err)
			fmt.Printf("%v\n", err)
		}
	case *net.UnixConn:
		if err := conn.CloseWrite(); err != nil {
			err = fmt.Errorf("(%s->%s): CloseWrite failed: %v", c.Conn.LocalAddr(), c.Conn.RemoteAddr(), err)
			fmt.Printf("%v\n", err)
		}

	}
	return c.Conn.Close()
}

type UnixConnListener struct {
	*net.UnixConn
}

func (u *UnixConnListener) Addr() net.Addr {
	return u.UnixConn.RemoteAddr()
}

func (u *UnixConnListener) Close() error {
	return u.UnixConn.Close()
}

func Get(via *net.UnixConn, filename string) (*os.File, error) {
	viaf, err := via.File()
	if err != nil {
		return nil, fmt.Errorf("1: %v", err)
	}
	socket := int(viaf.Fd())
	defer viaf.Close()
	return GetFd(socket, filename)
}

func GetFd(fd int, filename string) (*os.File, error) {
	// recvmsg
	buf := make([]byte, syscall.CmsgSpace(4))
	_, _, _, _, err := syscall.Recvmsg(fd, nil, buf, 0)
	if err != nil {
		return nil, fmt.Errorf("2: %v", err)
	}

	// parse control msgs
	msgs, err := syscall.ParseSocketControlMessage(buf)
	if err != nil {
		return nil, fmt.Errorf("3: %v, %v", err, buf)
	}

	fds, err := syscall.ParseUnixRights(&msgs[0])
	if err != nil {
		return nil, fmt.Errorf("4: %v", err)
	}
	// convert fds to files
	return os.NewFile(uintptr(fds[0]), filename), nil
}

func Put(via *net.UnixConn, file *os.File) error {
	viaf, err := via.File()
	if err != nil {
		return err
	}
	socket := int(viaf.Fd())
	defer viaf.Close()
	return PutFd(socket, file)
}

func PutFd(fd int, file *os.File) error {
	rights := syscall.UnixRights(int(file.Fd()))
	n, err := syscall.SendmsgN(fd, nil, rights, nil, 0)
	if err != nil {
		return err
	}

	if n != 0 {
		return fmt.Errorf("n(%d) != 0", n)
	}
	return nil
}

func (u *UnixConnListener) Accept() (net.Conn, error) {
	file, err := Get(u.UnixConn, "remote")
	if err != nil {
		return nil, fmt.Errorf("UnixConnListener: err: fd.Get: %v: %v->%v", err, u.UnixConn.LocalAddr(), u.UnixConn.RemoteAddr())
	}

	defer file.Close()
	fc, err := net.FileConn(file)
	if err != nil {
		return nil, fmt.Errorf("UnixConnListener: err: fileconn: %v", err)
	}

	return fc, nil
}

func CopyUnix(dst, src net.Conn) (err error) {
	var from, to *net.UnixConn
	switch s := src.(type) {
	case *CloseWriter:
		from = s.NetConn().(*net.UnixConn)
	case *net.UnixConn:
		from = s
	}
	switch d := dst.(type) {
	case *CloseWriter:
		to = d.NetConn().(*net.UnixConn)
	case *net.UnixConn:
		to = d
	}

	for {
		p := make([]byte, 0)
		oob := make([]byte, 24)
		_, _, _, _, err = from.ReadMsgUnix(p, oob)
		if err != nil {
			return
		}

		_, _, err = to.WriteMsgUnix(p, oob, nil)
		if err != nil {
			return
		}
	}
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
