package lib

import (
	"os/user"
	"strconv"
	"syscall"
)

type User struct {
	User  string `long:"user" description:"change to user on listen thread"`
	Group string `long:"group" description:"change to group on listen thread"`
	UID   int    `long:"uid" description:"change user on listen thread"`
	GID   int    `long:"gid" description:"change group on listen thread"`
}

func (u *User) Lookup() error {
	var uid, gid int
	if u.UID == 0 && u.User != "" {
		lu, err := user.Lookup(u.User)
		if err != nil {
			return err
		}
		uidInt32, err := strconv.ParseInt(lu.Uid, 10, 32)
		if err != nil {
			return err
		}
		gidInt32, err := strconv.ParseInt(lu.Gid, 10, 32)
		if err != nil {
			return err
		}

		uid = int(uidInt32)
		gid = int(gidInt32)
		u.User = ""
	}

	if u.GID == 0 && u.Group != "" {
		lg, err := user.LookupGroup(u.Group)
		if err != nil {
			return err
		}
		gidInt32, err := strconv.ParseInt(lg.Gid, 10, 32)
		if err != nil {
			return err
		}

		gid = int(gidInt32)
		u.Group = ""
	}

	u.UID, u.GID = uid, gid
	return nil
}

func (u *User) Switch() error {
	if err := u.Lookup(); err != nil {
		return err
	}

	if u.GID > 0 {
		if err := syscall.Setgroups([]int{}); err != nil {
			return err
		}

		if err := syscall.Setgid(u.GID); err != nil {
			return err
		}
	}

	if u.UID > 0 {
		if err := syscall.Setuid(u.UID); err != nil {
			return err
		}
	}

	return nil
}
