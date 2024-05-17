package main

import (
	"context"
	"fmt"
	"os"

	"github.com/isodude/gopipe/lib"
	"github.com/jessevdk/go-flags"

	"golang.org/x/sync/errgroup"
)

type Connection struct {
	Listen lib.Listen `group:"client" namespace:"listen"`

	Client lib.Client `group:"client" namespace:"client"`

	proxy lib.Proxy

	Debug bool `long:"debug"`
}

func main() {
	connections := []*Connection{}

	osArgs := [][]string{}
	i, j, k := 0, 0, ""
	for j, k = range os.Args {
		if k == "--next" {
			osArgs = append(osArgs, os.Args[i+1:j])
			i = j
		}
	}
	if i == 0 {
		osArgs = append(osArgs, os.Args)
	} else {
		osArgs = append(osArgs, os.Args[i+1:])
	}

	for _, k := range osArgs {
		connection := &Connection{}
		if _, err := flags.ParseArgs(connection, k); err != nil {
			if flags.WroteHelp(err) {
				return
			}

			panic(err)
		}
		if connection.Listen.ShouldFork && connection.Client.ShouldFork {
			connection.proxy = &lib.ForkListenForkClientProxy{}
		} else if connection.Listen.ShouldFork {
			connection.proxy = &lib.ForkListenProxy{}
		} else if connection.Client.ShouldFork {
			connection.proxy = &lib.ForkClientProxy{}
		} else if connection.Client.Addr.IsFd() {
			connection.proxy = &lib.UnixSendProxy{}
		} else if connection.Listen.Addr.IsFd() && connection.Listen.IncomingConn {
			connection.proxy = &lib.UnixDialProxy{}
		} else {
			connection.proxy = &lib.SimpleProxy{}
		}
		connections = append(connections, connection)
	}

	bCtx := context.Background()
	g, ctx := errgroup.WithContext(bCtx)

	for _, k := range connections {
		if k.Debug {
			fmt.Printf("Found: %s(%s) -> %s(%s)\n",
				k.Listen.Addr,
				k.Listen.NetNs.SystemdUnit,
				k.Client.Addr,
				k.Client.NetNs.SystemdUnit,
			)
		}
		k.Listen.Ctx = ctx
		k.Listen.NetNs.Ctx = ctx
		if !k.Listen.NetNs.Disable {
			k.Listen.NetNs.SetCurrent()
		}
		k.Listen.NetNs.Protocol = k.Listen.Protocol
		k.Client.Ctx = ctx
		k.Client.NetNs.Ctx = ctx
		if !k.Client.NetNs.Disable {
			k.Client.NetNs.SetCurrent()
		}
		k.Client.NetNs.Protocol = k.Listen.Protocol

		if k.Debug {
			k.Listen.Debug = true
			k.Client.Debug = true
		}

		if k.Listen.Debug {
			k.Listen.NetNs.Debug = true
			k.Listen.TLS.Debug = true
		}

		if k.Client.Debug {
			k.Client.NetNs.Debug = true
			k.Client.TLS.Debug = true
		}

		if err := k.Listen.TLS.TLSConfig(); err != nil {
			panic(err)
		}

		if err := k.Client.TLS.TLSConfig(); err != nil {
			panic(err)
		}
		f := func() error {
			err := k.proxy.Proxy(&k.Listen, &k.Client)
			if err != nil && k.Debug {
				fmt.Printf("Error: %s: %v\n", k.Listen.Addr, err)
			}
			return err
		}

		g.Go(f)
	}
	if err := g.Wait(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
