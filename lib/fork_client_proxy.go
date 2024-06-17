package lib

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"runtime"
	"syscall"

	"os/exec"

	"github.com/ftrvxmtrx/fd"
)

type ForkClientProxy struct {
	ClientProc *Proc
	ListenProc *Proc
	ClientCmd  *exec.Cmd
	ListenCmd  *exec.Cmd
	Ln         net.Listener
}

func (f *ForkClientProxy) listen(l *Listen) (ln net.Listener, err error) {
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

func (f *ForkClientProxy) dial(c *Client) (*net.UnixConn, error) {
	pipe := &Pipe{}
	conns, err := pipe.Unixpair()
	if err != nil {
		return nil, err
	}
	args := []string{fmt.Sprintf("--client.addr=%s", c.GetAddr()), "--listen.addr=FD:3", "--listen.conn"}
	args = append(args, c.TLS.Args("client.tls")...)
	//fmt.Printf("forking %s %v\n", os.Args[0], args)

	cmd := exec.CommandContext(c.Ctx, os.Args[0], args...)

	cloneflags, err := NewCloneflags()
	if err != nil {
		return nil, err
	}

	cloneflags.PrivateMounts = true
	cloneflags.PrivatePID = true
	if c.UID == 0 && c.GID == 0 {
		cloneflags.PrivateUsers = true
	}
	cloneflags.PrivateUTS = true
	// hangs child process
	// cloneflags.PrivateTLS = true
	cloneflags.PrivateIO = true
	cloneflags.PrivateIPC = true
	cloneflags.PrivateClock = true
	cloneflags.PrivateCGroup = true

	f.ClientProc = &Proc{Cloneflags: cloneflags}

	if err := f.ClientProc.SetUserGroup(c.User); err != nil {
		return nil, err
	}

	f.ClientProc.SetSysProcAttr(cmd)

	cmd.ExtraFiles, cmd.Stdout, cmd.Stderr = []*os.File{pipe.Files[1]}, os.Stdout, os.Stderr

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err, _ = c.NetNs.Enter()
	if err != nil {
		return nil, err
	}
	defer c.NetNs.Close()
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
	f.ClientCmd = cmd

	return uc, nil
}

func (f *ForkClientProxy) send(u *net.UnixConn, src net.Conn) error {
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

	return fd.Put(u, os.NewFile(uintptr(connFd), "remote"))
}

func (f *ForkClientProxy) Close() error {
	return f.Ln.Close()
}

func (f *ForkClientProxy) Proxy(l *Listen, c *Client) (err error) {
	f.Ln, err = f.listen(l)
	if err != nil {
		return
	}

	u, err := f.dial(c)
	if err != nil {
		return
	}
	defer f.ClientCmd.Cancel()

	// Make sure ln is closed if cmd exits
	go func() {
		if err := f.ClientCmd.Wait(); err != nil {
			if err, ok := err.(*exec.ExitError); ok {
				fmt.Printf("unable to start process: %v, %s", err, err.Stderr)
				return
			}
			fmt.Printf("unable to start process: %v", err)
		}
	}()

	var src net.Conn
	for {
		if src, err = f.Ln.Accept(); err != nil {
			return
		}

		if t, ok := src.(*tls.Conn); ok {
			src, err = TerminateTLS(t)
			if err != nil {
				src.Close()
				continue
			}
		}

		go f.send(u, src)
	}
}
