package main

import (
	"context"
    "os"

	"github.com/jessevdk/go-flags"
    "github.com/isodude/gopipe/lib"
)

type Args struct {
	Listen lib.Listen `group:"client" namespace:"listen"`

	Client lib.Client `group:"client" namespace:"client"`

	Debug bool `long:"debug"`
}

func main() {
	args := &Args{}
	if _, err := flags.ParseArgs(args, os.Args[1:]); err != nil {
		panic(err)
	}

	ctx := context.Background()
	args.Listen.Ctx = ctx
	args.Listen.NetNs.Ctx = ctx
	args.Client.Ctx = ctx
	args.Client.NetNs.Ctx = ctx

    if args.Debug {
        args.Listen.Debug = true
        args.Client.Debug = true
    }

    if args.Listen.Debug {
        args.Listen.NetNs.Debug = true
        args.Listen.TLS.Debug = true
    }

    if args.Client.Debug {
        args.Client.NetNs.Debug = true
        args.Client.TLS.Debug = true
    }

	if err := args.Listen.TLS.TLSConfig(); err != nil {
		panic(err)
	}

	if err := args.Client.TLS.TLSConfig(); err != nil {
		panic(err)
	}

    if err := args.Listen.Listen(&args.Client); err != nil {
	   	panic(err)
    }
}
