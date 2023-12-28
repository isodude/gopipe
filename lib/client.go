package lib

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
)

type Client struct {
	Debug    bool             `long:"debug"`
	TLS      ClientTLS        `group:"tls" namespace:"tls"`
	NetNs    NetworkNamespace `group:"netns" namespace:"netns"`
	Addr     string           `long:"addr" description:"connect to address"`
	SourceIP string           `long:"source-ip" description:"IP used as source address"`

	Ctx context.Context
}

func (c *Client) Dial(from net.Conn) {
	defer from.Close()
	var err error
	var to net.Conn
	var triedAgain bool

again:
	if c.TLS.config != nil {
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

	func(c [2]net.Conn) {
		defer c[0].Close()
		defer c[1].Close()

		var wg sync.WaitGroup
		wg.Add(2)

		f := func(c [2]net.Conn) {
			defer wg.Done()
			io.Copy(c[0], c[1])
			// Signal peer that no more data is coming.
			if tcpConn, ok := c[0].(*net.TCPConn); ok {
				tcpConn.CloseWrite()
			}
		}
		go f(c)
		go f([2]net.Conn{c[1], c[0]})

		wg.Wait()
	}([2]net.Conn{to, from})

	return
}
