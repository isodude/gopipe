package lib

import (
    "context"
    "crypto/tls"
    "fmt"
    "net"
    "os/user"
    "runtime"
    "strings"
    "strconv"
    "syscall"

    "github.com/coreos/go-systemd/v22/activation"
)

type Listen struct {
	Debug bool             `long:"debug"`
	TLS   ListenTLS        `group:"tls" namespace:"tls"`
	NetNs NetworkNamespace `group:"netns" namespace:"netns"`
	Addr  string           `long:"addr" description:"listen on address"`
    User  string       `long:"user" description:"change to user on listen thread"`
    Group string       `long:"group" description:"change to group on listen thread"`
    UID         int    `long:"uid" description:"change user on listen thread"`
    GID         int    `long:"gid" description:"change group on listen thread"`

	Ctx      context.Context
	listener net.Listener
	client   *Client
}

func (l *Listen) SetClient(client *Client) {
	l.client = client
}

func (l *Listen) Listen(client *Client) (err error) {
	runtime.LockOSThread()
    defer runtime.UnlockOSThread()
    var ok bool
	if err, ok = l.NetNs.Enter(); err != nil {
		return
	}
    if !ok {
        if l.Debug {
            fmt.Printf("Not entering namespace\n")
        }
    }

    if strings.HasPrefix(l.Addr, "FD:") {
        addr := l.Addr[3:]
        var fd int
        fd, err = strconv.Atoi(addr)
        if err == nil {
            var fds []net.Listener
            if l.TLS.config != nil {
                fds, err = activation.TLSListeners(l.TLS.config)
            } else {
                fds, err = activation.Listeners()
            }
            if err != nil {
                return
            }
            // fd == 3, len(fds) == 1
            if len(fds) < fd - 2 {
                return fmt.Errorf("not enough fds from systemd, got %d wanted %d", len(fds), fd-2)
            }
            l.listener = fds[fd-3]
        } else {
            return fmt.Errorf("not implemented yet: %s", addr)
         }
    } else if l.TLS.config != nil {
		l.listener, err = tls.Listen("tcp", l.Addr, l.TLS.config)
	} else {
		l.listener, err = net.Listen("tcp", l.Addr)
	}
	if err != nil {
		return
	}

	var from net.Conn

    if err = client.NetNs.ChangeEveryThread(); err != nil {
        return err
    }

    if l.UID == 0 && l.User != "" {
        user, err := user.Lookup(l.User)
        if err != nil {
            return err
        }
        uid, err := strconv.ParseInt(user.Uid, 10, 32)
        if err != nil {
            return err
        }
        gid, err := strconv.ParseInt(user.Gid, 10, 32)
        if err != nil {
            return err
        }

        l.UID = int(uid)
        l.GID = int(gid)
    }
    if l.GID == 0 && l.Group != "" {
        group, err := user.LookupGroup(l.Group)
        if err != nil {
            return err
        }
        gid, err := strconv.ParseInt(group.Gid, 10, 32)
        if err != nil {
            return err
        }

        l.GID = int(gid)
    }

    if l.GID > 0 {
        if err = syscall.Setgroups([]int{}); err != nil {
            return err
        }

        if err = syscall.Setgid(l.GID); err != nil {
            return err
        }
    }


    if l.UID > 0 {
        if err = syscall.Setuid(l.UID); err != nil {
            return err
        }
    }

	for {
		from, err = l.listener.Accept()
		if err != nil {
			return
		}

		go client.Dial(from)
	}

	return
}
