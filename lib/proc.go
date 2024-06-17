package lib

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

type Proc struct {
	Ctx    context.Context
	Chroot string

	ShouldFork bool `long:"fork" description:"fork process"`

	Cloneflags *Cloneflags
	Uid        int
	Gid        int
}

func (p *Proc) SetUserGroup(u *User) error {
	if err := u.Lookup(); err != nil {
		return err
	}
	p.Uid, p.Gid = u.UID, u.GID
	return nil
}

type OSFile interface {
	File() (*os.File, error)
}

func (p *Proc) SetSysProcAttr(c *exec.Cmd) {
	c.SysProcAttr = p.Cloneflags.Set()
	if p.Uid > 0 || p.Gid > 0 {
		c.SysProcAttr.Credential = &syscall.Credential{}
	}
	if p.Uid > 0 {
		c.SysProcAttr.Credential.Uid = uint32(p.Uid)
	}
	if p.Gid > 0 {
		c.SysProcAttr.Credential.Gid = uint32(p.Gid)
	}
}

func ForkUnixConn(ctx context.Context, user *User, netns *NetworkNamespace, bin string, args ...string) (*exec.Cmd, *net.UnixConn, error) {
	cmd := exec.CommandContext(ctx, bin, args...)

	cloneflags, err := NewCloneflags()
	if err != nil {
		return nil, nil, err
	}

	if err := user.Lookup(); err != nil {
		return nil, nil, err
	}

	cloneflags.PrivateMounts = true
	cloneflags.PrivatePID = true
	if user.UID == 0 && user.GID == 0 {
		cloneflags.PrivateUsers = true
	}
	cloneflags.PrivateUTS = true
	// hangs child process
	// cloneflags.PrivateTLS = true
	cloneflags.PrivateIO = true
	cloneflags.PrivateIPC = true
	cloneflags.PrivateClock = true
	cloneflags.PrivateCGroup = true

	p := &Proc{Cloneflags: cloneflags}
	if err := p.SetUserGroup(user); err != nil {
		return nil, nil, err
	}
	p.SetSysProcAttr(cmd)

	conns, err := UnixPipe()
	if err != nil {
		return nil, nil, err
	}

	fc, _ := conns[1].(*net.UnixConn).File()
	cmd.ExtraFiles, cmd.Stdout, cmd.Stderr = []*os.File{fc}, os.Stdout, os.Stderr

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err, _ = netns.Enter()
	if err != nil {
		return nil, nil, err
	}
	defer netns.Close()
	if err := cmd.Start(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return nil, nil, fmt.Errorf("unable to start process: %v, %s, %s", err, err.Stderr, cmd.Environ())
		}
		return nil, nil, fmt.Errorf("unable to start process: %v", err)
	}

	uc, ok := conns[0].(*net.UnixConn)
	if !ok {
		return nil, nil, fmt.Errorf("unable to convert conn to unixconn")
	}

	return cmd, uc, nil
}
