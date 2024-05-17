package lib

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"

	"os/exec"
)

type ForkListenProxy struct {
	*Proc
	Cmd *exec.Cmd
}

func (f *ForkListenProxy) listen(l *Listen) (net.Listener, error) {
	pipe := &Pipe{}
	conns, err := pipe.Unixpair()
	if err != nil {
		return nil, err
	}

	args := []string{fmt.Sprintf("--listen.addr=%s", l.GetAddr()), "--client.addr=FD:3"}
	args = append(args, l.TLS.Args("listen.tls")...)

	cmd := exec.CommandContext(l.Ctx, os.Args[0], args...)

	cloneflags, err := NewCloneflags()
	if err != nil {
		return nil, err
	}

	cloneflags.PrivateMounts = true
	cloneflags.PrivatePID = true
	if l.UID == 0 && l.GID == 0 {
		cloneflags.PrivateUsers = true
	}
	cloneflags.PrivateUTS = true
	// hangs child process
	// cloneflags.PrivateTLS = true
	cloneflags.PrivateIO = true
	cloneflags.PrivateIPC = true
	cloneflags.PrivateClock = true
	cloneflags.PrivateCGroup = true

	f.Proc = &Proc{Cloneflags: cloneflags}
	if err := f.SetUserGroup(l.User); err != nil {
		return nil, err
	}
	f.SetSysProcAttr(cmd)

	cmd.ExtraFiles, cmd.Stdout, cmd.Stderr = []*os.File{pipe.Files[1]}, os.Stdout, os.Stderr

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err, _ = l.NetNs.Enter()
	if err != nil {
		return nil, err
	}
	defer l.NetNs.Close()
	if err := cmd.Start(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("unable to start process: %v, %s, %s", err, err.Stderr, cmd.Environ())
		}
		return nil, fmt.Errorf("unable to start process: %v", err)
	}

	uc, ok := conns[0].(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("unable to convert conn to unixconn")
	}
	f.Cmd = cmd
	return &UnixConnListener{uc}, nil
}

func (f *ForkListenProxy) dial(c *Client) (conn net.Conn, err error) {
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
		if err == io.EOF {
			err = nil
		}
	}

	return
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
