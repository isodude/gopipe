package lib

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/coreos/go-systemd/v22/activation"
)

type Addr struct {
	Addr string `long:"addr" description:"connect to address"`
}

func (a *Addr) Conn() (net.Conn, error) {
	file, err := a.File()
	if err != nil {
		return nil, err
	}

	defer file.Close()
	return net.FileConn(file)
}

func (a *Addr) File() (*os.File, error) {
	fd, err := a.Fd()
	if err != nil {
		return nil, err
	}

	f := os.NewFile(uintptr(fd), "client")
	return f, nil
}

func (a *Addr) Fd() (int, error) {
	if !a.IsFd() {
		return 0, fmt.Errorf("not an FD: %s", a.Addr)
	}

	return strconv.Atoi(a.Addr[3:])
}

func (a *Addr) IsFd() bool {
	return strings.HasPrefix(a.Addr, "FD:")
}

func (a *Addr) GetAddr() string {
	return a.Addr
}

func (a *Addr) GetListener(tlsConfig *tls.Config) (net.Listener, error) {
	// look for incoming listener from parent process, user used --listen.fork
	fd, err := a.Fd()
	if err != nil {
		return nil, fmt.Errorf("not implemented yet: %v", err)
	}
	// activation requires that LISTEN_PID is set correctly
	// it's tough to know beforehand while cloning
	if os.Getenv("FIX_LISTEN_PID") != "" {
		os.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))
	}

	var fds []net.Listener
	if tlsConfig != nil {
		fds, err = activation.TLSListeners(tlsConfig)
	} else {
		fds, err = activation.Listeners()
	}
	if err != nil {
		return nil, err
	}
	// fd == 3, len(fds) == 1
	// fd == 3, len(fds) == 2
	if len(fds) < fd-2 {
		return nil, fmt.Errorf("not enough fds from systemd, got %d wanted %d", len(fds), fd-2)
	}
	return fds[fd-3], nil
}
