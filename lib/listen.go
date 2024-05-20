package lib

import (
	"context"
)

type Listen struct {
	*Addr
	*User
	*Proc
	Debug    bool             `long:"debug"`
	TLS      ListenTLS        `group:"tls" namespace:"tls"`
	NetNs    NetworkNamespace `group:"netns" namespace:"netns"`
	Protocol string           `long:"protocol" default:"tcp" choice:"unix" choice:"unixgram" choice:"udp" choice:"tcp" description:"The protocol to connect with"`

	IncomingConn bool `long:"conn" description:"Accept conns from parent"`

	Ctx    context.Context
	client *Client
}

func (l *Listen) SetClient(client *Client) {
	l.client = client
}
