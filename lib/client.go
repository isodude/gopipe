package lib

import (
	"context"
	"time"
)

type Client struct {
	*Addr
	*User
	*Proc
	Debug    bool             `long:"debug"`
	TLS      ClientTLS        `group:"tls" namespace:"tls"`
	NetNs    NetworkNamespace `group:"netns" namespace:"netns"`
	SourceIP string           `long:"source-ip" description:"IP used as source address"`
	Protocol string           `long:"protocol" default:"tcp" choice:"unix" choice:"unixgram" choice:"udp" choice:"tcp" description:"The protocol to connect with"`
	Timeout  time.Duration    `long:"timeout" default:"5s" description:"The connect timeout"`
	Ctx      context.Context
	Cancel   context.CancelCauseFunc
}
