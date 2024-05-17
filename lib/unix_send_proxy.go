package lib

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
)

type UnixSendProxy struct {
}

func (s *UnixSendProxy) listen(l *Listen) (ln net.Listener, err error) {
	if l.IsFd() {
		ln, err = l.GetListener(l.TLS.config)
		if err != nil {
			return
		}
	} else {
		ln, err = net.Listen(l.Protocol, l.GetAddr())
		if err != nil {
			return
		}
	}

	if l.TLS.config != nil {
		ln = tls.NewListener(ln, l.TLS.config)
	}

	return
}

/*
func (f *UnixSendProxy) send(c *Client, src net.Conn) (err error) {

	// It would make sense to close the file here.
	// However this is the socket to talk to the main process, if it's closed,
	// it will be replaced with e.g. a net.TCPConn.
	// cf.Close()

	rawConn, err := src.(syscall.Conn).SyscallConn()
	if err != nil {
		return err
	}

	var connFd int
	if err = rawConn.Control(func(fd uintptr) {
		connFd = int(fd)
	}); err != nil {
		return err
	}

	return Put(uc, os.NewFile(uintptr(connFd), "remote"))
}*/

func (f *UnixSendProxy) Proxy(l *Listen, c *Client) (err error) {
	var ln net.Listener
	ln, err = f.listen(l)
	if err != nil {
		return
	}
	uc, err := c.Fd()
	if err != nil {
		return err
	}
	var src net.Conn
	for {
		if src, err = ln.Accept(); err != nil {
			return
		}

		if t, ok := src.(*tls.Conn); ok {
			p := &Pipe{}
			conns, err := p.Unixpair()
			if err != nil {
				fmt.Printf("error: %v\n", err)
				continue
			}
			go func() {
				go func() {
					io.Copy(conns[0], t)
					(&CloseWriter{conns[0]}).Close()
				}()
				io.Copy(t, conns[0])
				(&CloseWriter{t}).Close()
			}()
			uf, err := conns[1].(*net.UnixConn).File()
			if err != nil {
				fmt.Printf("error: %v\n", err)
				continue
			}
			if err := PutFd(uc, uf); err != nil {
				fmt.Printf("error: %v\n", err)
				continue
			}
		} else {
			uf, err := src.(*net.TCPConn).File()
			if err != nil {
				continue
			}
			if err := PutFd(uc, uf); err != nil {
				fmt.Printf("error: %v\n", err)
				continue
			}
		}
	}
}
