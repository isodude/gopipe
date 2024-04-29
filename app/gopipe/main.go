package main

import (
	"context"
	"os"

	"github.com/isodude/gopipe/lib"
	"github.com/jessevdk/go-flags"

	"golang.org/x/sync/errgroup"
)

type Connection struct {
	Listen lib.Listen `group:"client" namespace:"listen"`

	Client lib.Client `group:"client" namespace:"client"`

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
		connections = append(connections, connection)
	}

	bCtx := context.Background()
	g, ctx := errgroup.WithContext(bCtx)

	for _, k := range connections {
		k.Listen.Ctx = ctx
		k.Listen.NetNs.Ctx = ctx
		k.Client.Ctx = ctx
		k.Client.NetNs.Ctx = ctx

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

		g.Go(func() error {
			return k.Listen.Listen(&k.Client)
		})
	}
	g.Wait()
}
