package lib

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

type Client struct {
	Debug    bool             `long:"debug"`
	TLS      ClientTLS        `group:"tls" namespace:"tls"`
	NetNs    NetworkNamespace `group:"netns" namespace:"netns"`
	Addr     string           `long:"addr" description:"connect to address"`
	SourceIP string           `long:"source-ip" description:"IP used as source address"`

	Ctx context.Context
}

func (c *Client) Conn() (net.Conn, error) {
	file, err := c.File()
	if err != nil {
		return nil, err
	}

	defer file.Close()
	return net.FileConn(file)
}

func (c *Client) File() (*os.File, error) {
	fd, err := c.Fd()
	if err != nil {
		return nil, err
	}

	f := os.NewFile(uintptr(fd), "client")
	return f, nil
}

func (c *Client) IsFd() bool {
	if !strings.HasPrefix(c.Addr, "FD:") {
		return false
	}

	return true
}

func (c *Client) Fd() (int, error) {
	if !c.IsFd() {
		return 0, fmt.Errorf("Not an FD: %s", c.Addr)
	}

	return strconv.Atoi(c.Addr[3:])
}

func (c *Client) Dial(from net.Conn) {
	defer from.Close()
	var err error
	var to net.Conn
	var triedAgain bool

again:
	if c.IsFd() {
		connFd, err := ConnFd(from.(*net.TCPConn))
		if err != nil {
			fmt.Printf("dial: err: %v\n", err)
			return
		}

		rights := unix.UnixRights(connFd)

		fd, err := c.Fd()
		if err != nil {
			fmt.Printf("dial: err: %v\n", err)
			return
		}

		unix.Sendmsg(fd, nil, rights, nil, 0)
		return
	} else if c.TLS.config != nil {
		to, err = tls.DialWithDialer(c.NetNs.Dialer(c.SourceIP), "tcp", c.Addr, c.TLS.config)
	} else {
		to, err = c.NetNs.Dialer(c.SourceIP).Dial("tcp", c.Addr)
	}

	if err != nil {
		if !triedAgain {
			triedAgain = true
			goto again
		}
		if err != io.EOF {
			fmt.Printf("dial: err: %v\n", err)
		}
		return
	}

	CopyDuplex(to, from)

	return
}
