package lib

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"

	"os/exec"
)

type ForkListenForkClientProxy struct {
	ClientProc *Proc
	ListenProc *Proc
	ClientCmd  *exec.Cmd
	ListenCmd  *exec.Cmd
	Ctx        context.Context
	Cancel     context.CancelCauseFunc
}

func (f *ForkListenForkClientProxy) listen(l *Listen) (*net.UnixConn, error) {
	pipe := &Pipe{}
	conns, err := pipe.Unixpair()
	if err != nil {
		return nil, err
	}

	args := []string{fmt.Sprintf("--listen.addr=%s", l.GetAddr()), "--client.addr=FD:3"}
	args = append(args, l.TLS.Args("listen.tls")...)
	args = append(args, "--listen.netns.disable", "--client.netns.disable")
	fmt.Printf("forking %s %v\n", os.Args[0], args)

	cmd := exec.CommandContext(f.Ctx, os.Args[0], args...)

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

	f.ListenProc = &Proc{Cloneflags: cloneflags}
	if err := f.ListenProc.SetUserGroup(l.User); err != nil {
		return nil, err
	}
	f.ListenProc.SetSysProcAttr(cmd)

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
	f.ListenCmd = cmd
	return uc, nil
}

func (f *ForkListenForkClientProxy) dial(c *Client) (*net.UnixConn, error) {
	pipe := &Pipe{}
	conns, err := pipe.Unixpair()
	if err != nil {
		return nil, err
	}
	args := []string{fmt.Sprintf("--client.addr=%s", c.GetAddr()), "--listen.addr=FD:3", "--listen.conn"}
	args = append(args, c.TLS.Args("client.tls")...)
	args = append(args, "--listen.netns.disable", "--client.netns.disable")
	fmt.Printf("forking %s %v\n", os.Args[0], args)

	cmd := exec.CommandContext(f.Ctx, os.Args[0], args...)

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

func (f *ForkListenForkClientProxy) Proxy(l *Listen, c *Client) error {
	f.Ctx, f.Cancel = context.WithCancelCause(l.Ctx)
	var src, dst net.Conn
	var err error
	src, err = f.listen(l)
	if err != nil {
		return err
	}
	defer f.ListenCmd.Cancel()

	defer func() {
		if err := src.Close(); err != nil {
			fmt.Printf("unable to close: %v\n", err)
		}
	}()

	// Make sure ln is closed if cmd exits
	go func() {
		if err := f.ListenCmd.Wait(); err != nil {
			if er, ok := err.(*exec.ExitError); ok {
				err = fmt.Errorf("process exited: %v, %s", er, er.Stderr)
			}
			f.Cancel(err)
		}
	}()

	dst, err = f.dial(c)
	if err != nil {
		return err
	}
	defer f.ClientCmd.Cancel()

	// Make sure ln is closed if cmd exits
	go func() {
		if err := f.ClientCmd.Wait(); err != nil {
			if er, ok := err.(*exec.ExitError); ok {
				err = fmt.Errorf("process exited: %v, %s", er, er.Stderr)
			}
			f.Cancel(err)
		}
	}()

	defer func() {
		if err := dst.Close(); err != nil {
			fmt.Printf("unable to close: %v\n", err)
		}
	}()

	go func() {
		f.Cancel(CopyUnix(&CloseWriter{dst}, &CloseWriter{src}))
	}()

	<-f.Ctx.Done()
	return f.Ctx.Err()
}
