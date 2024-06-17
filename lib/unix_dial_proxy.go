package lib

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

type UnixDialProxy struct {
	Ln net.Listener
}

func (s *UnixDialProxy) listen(l *Listen) (ln net.Listener, err error) {
	if !strings.HasPrefix(l.Addr.GetAddr(), "FD:") {
		return nil, fmt.Errorf("addr not a fd: %v", l.Addr)
	}
	i, err := strconv.Atoi(l.Addr.GetAddr()[3:])
	if err != nil {
		return nil, err
	}
	cf := os.NewFile(uintptr(i), "client")

	u, err := net.FileConn(cf)
	if err != nil {
		return nil, err
	}

	uc, ok := u.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("unable to convert conn to unixconn")
	}

	return &UnixConnListener{uc}, nil
}

func (f *UnixDialProxy) dial(c *Client) (conn net.Conn, err error) {
	return (&Dialer{
		NetNs:     &c.NetNs,
		Timeout:   c.Timeout,
		TLSConfig: c.TLS.config,
		SourceIP:  c.SourceIP,
	}).DialContext(c.Ctx, c.Protocol, c.GetAddr())
}

func (f *UnixDialProxy) Close() error {
	return f.Ln.Close()
}

func (f *UnixDialProxy) Proxy(l *Listen, c *Client) (err error) {
	f.Ln, err = f.listen(l)
	if err != nil {
		return
	}

	var src, dst net.Conn

	for {
		if src, err = f.Ln.Accept(); err != nil {
			return
		}

		go func() {
			defer func() {
				if err := src.Close(); err != nil {
					fmt.Printf("unable to close: %v\n", err)
				}
			}()
			if dst, err = f.dial(c); err != nil {
				fmt.Printf("unable to dial: %v\n", err)
				return
			}

			src = &CloseWriter{src}
			go func() {
				defer func() {
					cw := &CloseWriter{dst}
					if err := cw.Close(); err != nil {
						fmt.Printf("unable to close: %v\n", err)
					}
				}()
				io.Copy(dst, src)
			}()
			io.Copy(src, dst)
		}()
	}
}
