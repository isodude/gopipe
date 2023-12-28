package main

import (
	"context"
	"os"

	"github.com/isodude/gopipe/lib"
	"github.com/jessevdk/go-flags"
)

type Args struct {
	Listen lib.Listen `group:"client" namespace:"listen"`

	Client lib.Client `group:"client" namespace:"client"`

	Debug bool `long:"debug"`
}

func main() {
	args := &Args{}
	p := flags.NewParser(args, flags.Default)
	if len(os.Args) == 1 {
		p.WriteHelp(os.Stderr)
		return
	}

	if _, err := p.Parse(); err != nil {
		if flags.WroteHelp(err) {
			return
		}

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
