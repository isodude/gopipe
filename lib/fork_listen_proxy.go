package lib

import (
	"fmt"
	"io"
	"net"
	"os"

	"os/exec"
)

type ForkListenProxy struct {
	*Proc
	Cmd *exec.Cmd
}

func (f *ForkListenProxy) listen(l *Listen) (net.Listener, error) {
	args := []string{fmt.Sprintf("--listen.addr=%s", l.GetAddr()), "--client.addr=FD:3"}
	args = append(args, l.TLS.Args("listen.tls")...)

	cmd, uc, err := ForkUnixConn(l.Ctx, l.User, &l.NetNs, os.Args[0], args...)
	if err != nil {
		return nil, err
	}

	f.Cmd = cmd
	return &UnixConnListener{uc}, nil
}

func (f *ForkListenProxy) dial(c *Client) (conn net.Conn, err error) {
	return (&Dialer{
		NetNs:     &c.NetNs,
		Timeout:   c.Timeout,
		TLSConfig: c.TLS.config,
		SourceIP:  c.SourceIP,
	}).DialContext(c.Ctx, c.Protocol, c.GetAddr())
}

func (f *ForkListenProxy) Close() error {
	return nil
}

func (f *ForkListenProxy) Proxy(l *Listen, c *Client) (err error) {
	var ln net.Listener
	ln, err = f.listen(l)
	if err != nil {
		return
	}
	defer f.Cmd.Cancel()

	// Make sure ln is closed if cmd exits
	go func() {
		if err := f.Cmd.Wait(); err != nil {
			if err, ok := err.(*exec.ExitError); ok {
				fmt.Printf("unable to start process: %v, %s", err, err.Stderr)
			}
			fmt.Printf("unable to start process: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	var src, dst net.Conn
	for {
		if src, err = ln.Accept(); err != nil {
			return
		}

		go func() {
			src = &CloseWriter{src}
			defer func() {
				if err := src.Close(); err != nil {
					fmt.Printf("unable to close: %v\n", err)
				}
			}()
			if dst, err = f.dial(c); err != nil {
				fmt.Printf("unable to dial: %v\n", err)
				return
			}
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
