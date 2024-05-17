package lib

import (
	"context"
	"os"
	"os/exec"
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

func (p *Proc) listenerToFile(l OSFile) (*os.File, error) {
	return l.File()
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
