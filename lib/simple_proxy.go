package lib

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
)

type SimpleProxy struct {
}

func (s *SimpleProxy) listen(l *Listen) (ln net.Listener, err error) {
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

func (s *SimpleProxy) dial(c *Client) (conn net.Conn, err error) {
	ctx, cancel := context.WithTimeout(c.Ctx, c.Timeout)
	defer cancel()
	dialer, err := c.NetNs.Dialer(c.SourceIP, c.Timeout)
	if err != nil {
		return nil, err
	}
	if c.TLS.config != nil {
		conn, err = tls.DialWithDialer(dialer, c.Protocol, c.GetAddr(), c.TLS.config)
	} else {
		conn, err = dialer.DialContext(ctx, c.Protocol, c.GetAddr())
	}

	if err != nil {
		/*		if !triedAgain {
				triedAgain = true
				goto again
			}*/
		if err == io.EOF {
			err = nil
		}
	}

	return
}

func (s *SimpleProxy) Proxy(l *Listen, c *Client) (err error) {
	var ln net.Listener
	ln, err = s.listen(l)
	if err != nil {
		return
	}
	var src, dst net.Conn
	for {
		if src, err = ln.Accept(); err != nil {
			return
		}

		go func() {
			defer func() {
				if err := src.Close(); err != nil {
					fmt.Printf("unable to close: %v\n", err)
				}
			}()
			if dst, err = s.dial(c); err != nil {
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
