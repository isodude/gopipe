package lib

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

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

	bin := os.Args[0]
	args := []string{}
	if e := os.Getenv("CMD_LISTEN_FORK_ARGS"); e != "" {
		args = append(args, strings.Split(e, " ")...)
	}
	args = append(args, fmt.Sprintf("--listen.addr=%s", l.GetAddr()), "--client.addr=FD:3")
	args = append(args, l.TLS.Args("listen.tls")...)
	args = append(args, "--listen.netns.disable", "--client.netns.disable")

	cmd, uc, err := ForkUnixConn(l.Ctx, l.User, &l.NetNs, bin, args...)
	if err != nil {
		return nil, err
	}
	f.ListenCmd = cmd
	return uc, nil
}

func (f *ForkListenForkClientProxy) dial(c *Client, conn *net.UnixConn, ch chan struct{}) error {
	closed := false
	defer func() {
		if !closed {
			close(ch)
		}
	}()
	cmdBin := os.Args[0]
	args := []string{}
	if e := os.Getenv("CMD_CLIENT_FORK_ARGS"); e != "" {
		args = append(args, strings.Split(e, " ")...)
	}
	args = append(args, fmt.Sprintf("--client.addr=%s", c.GetAddr()), "--listen.addr=FD:3", "--listen.conn")
	args = append(args, c.TLS.Args("client.tls")...)
	args = append(args, "--listen.netns.disable", "--client.netns.disable")
	//fmt.Printf("forking %s %v\n", cmdBin, args)

	f.ClientCmd = exec.CommandContext(f.Ctx, cmdBin, args...)

	cloneflags, err := NewCloneflags()
	if err != nil {
		return err
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
		return err
	}

	f.ClientProc.SetSysProcAttr(f.ClientCmd)

	fc, _ := conn.File()
	f.ClientCmd.ExtraFiles, f.ClientCmd.Stdout, f.ClientCmd.Stderr = []*os.File{fc}, os.Stdout, os.Stderr

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err, _ = c.NetNs.Enter()
	if err != nil {
		return err
	}
	defer c.NetNs.Close()
	if err := f.ClientCmd.Start(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("unable to start process: %v, %s, %s", err, err.Stderr, f.ClientCmd.Environ())
		}
		return fmt.Errorf("unable to start process: %v", err)
	}
	close(ch)
	closed = true

	return f.ClientCmd.Wait()
}

func (f *ForkListenForkClientProxy) Close() error {
	f.Cancel(nil)
	return nil
}

func (f *ForkListenForkClientProxy) Proxy(l *Listen, c *Client) error {
	f.Ctx, f.Cancel = context.WithCancelCause(l.Ctx)

	var src net.Conn
	var err error
	listenCh := make(chan struct{})

	go func() {
		src, err = f.listen(l)
		if err != nil {
			f.Cancel(err)
			close(listenCh)
			return
		}
		close(listenCh)
		defer src.Close()

		if err := f.ListenCmd.Wait(); err != nil {
			if er, ok := err.(*exec.ExitError); ok {
				err = fmt.Errorf("process exited: %v, %s", er, er.Stderr)
			}
			f.Cancel(err)
		}
	}()

	clientCh := make(chan struct{})

	go func() {
		select {
		case <-listenCh:
			defer f.ListenCmd.Process.Signal(os.Interrupt)
			break
		case <-f.Ctx.Done():
			return
		}
		defer src.Close()
		f.Cancel(f.dial(c, src.(*net.UnixConn), clientCh))
	}()

	<-clientCh
	if f.ClientCmd != nil {
		defer f.ClientCmd.Process.Signal(os.Interrupt)
	}

	<-f.Ctx.Done()
	return f.Ctx.Err()
}
