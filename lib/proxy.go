package lib

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"time"
)

type Proxy interface {
	Proxy(*Listen, *Client) error
	Close() error
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

func Get(via *net.UnixConn, filename string) (*os.File, error) {
	viaf, err := via.File()
	if err != nil {
		return nil, fmt.Errorf("file: %v", err)
	}
	defer viaf.Close()
	return GetFd(int(viaf.Fd()), filename)
}

func GetFd(fd int, filename string) (*os.File, error) {
	// recvmsg
	buf := make([]byte, syscall.CmsgSpace(4))
	_, _, _, _, err := syscall.Recvmsg(fd, nil, buf, 0)
	if err != nil {
		return nil, fmt.Errorf("recvmsg: %v", err)
	}

	// parse control msgs
	msgs, err := syscall.ParseSocketControlMessage(buf)
	if err != nil {
		return nil, fmt.Errorf("parsesocketcontrolmessage: %v, %d", err, fd)
	}

	fds, err := syscall.ParseUnixRights(&msgs[0])
	if err != nil {
		return nil, fmt.Errorf("parseunixrights: %v", err)
	}
	// convert fds to files
	return os.NewFile(uintptr(fds[0]), filename), nil
}

func Put(via *net.UnixConn, file *os.File) error {
	viaf, err := via.File()
	if err != nil {
		return err
	}
	defer viaf.Close()
	return PutFd(int(viaf.Fd()), file)
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

// Terminates TLS with two go routines.
// Will close conn when io.Copy is ended.
// The returned net.Conn is an actual UnixConn can
// be passed over unix sockets.
func TerminateTLS(conn *tls.Conn) (net.Conn, error) {
	conns, err := UnixPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		go func() {
			io.Copy(conns[0], conn)
			conns[0].Close()
		}()
		io.Copy(conn, conns[0])
		conn.Close()
	}()
	return conns[1], nil
}

type Dialer struct {
	NetNs     *NetworkNamespace
	SourceIP  string
	Timeout   time.Duration
	TLSConfig *tls.Config
}

func (d *Dialer) DialContext(ctx context.Context, protocol string, addr string) (conn net.Conn, err error) {
	ctx, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()
	dialer := &net.Dialer{}
	if d.NetNs != nil {
		dialer, err = d.NetNs.Dialer(d.SourceIP, d.Timeout)
		if err != nil {
			return nil, err
		}
	}
	if d.TLSConfig != nil {
		conn, err = tls.DialWithDialer(dialer, protocol, addr, d.TLSConfig)
	} else {
		conn, err = dialer.DialContext(ctx, protocol, addr)
	}

	if err != nil {
		if err == io.EOF {
			err = nil
		}
	}

	return
}
