package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"

	"github.com/cybozu-go/sabakan/client"
	"github.com/google/subcommands"
)

type assetsCmd struct{}

func (c assetsCmd) SetFlags(f *flag.FlagSet) {}

func (c assetsCmd) Execute(ctx context.Context, f *flag.FlagSet) subcommands.ExitStatus {
	cmdr := newCommander(f, "assets")
	cmdr.Register(assetsIndexCommand(), "")
	cmdr.Register(assetsInfoCommand(), "")
	cmdr.Register(assetsUploadCommand(), "")
	cmdr.Register(assetsDeleteCommand(), "")
	return cmdr.Execute(ctx)
}

func assetsCommand() subcommands.Command {
	return subcmd{
		assetsCmd{},
		"assets",
		"manage assets",
		"assets ACTION ...",
	}
}

type assetsIndexCmd struct{}

func (c assetsIndexCmd) SetFlags(f *flag.FlagSet) {}

func (c assetsIndexCmd) Execute(ctx context.Context, f *flag.FlagSet) subcommands.ExitStatus {
	if f.NArg() != 0 {
		f.Usage()
		return client.ExitUsageError
	}

	index, errStatus := client.AssetsIndex(ctx)
	if errStatus != nil {
		return handleError(errStatus)
	}

	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	err := e.Encode(index)
	return handleError(err)
}

func assetsIndexCommand() subcommands.Command {
	return subcmd{
		assetsIndexCmd{},
		"index",
		"get index of assets",
		"index",
	}
}

type assetsInfoCmd struct{}

func (c assetsInfoCmd) SetFlags(f *flag.FlagSet) {}

func (c assetsInfoCmd) Execute(ctx context.Context, f *flag.FlagSet) subcommands.ExitStatus {
	if f.NArg() != 1 {
		f.Usage()
		return client.ExitUsageError
	}

	asset, errStatus := client.AssetsInfo(ctx, f.Arg(0))
	if errStatus != nil {
		return handleError(errStatus)
	}

	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	err := e.Encode(asset)
	return handleError(err)
}

func assetsInfoCommand() subcommands.Command {
	return subcmd{
		assetsInfoCmd{},
		"info",
		"get meta data of asset",
		"info NAME",
	}
}

type assetsUploadCmd struct{}

func (c assetsUploadCmd) SetFlags(f *flag.FlagSet) {}

func (c assetsUploadCmd) Execute(ctx context.Context, f *flag.FlagSet) subcommands.ExitStatus {
	if f.NArg() != 2 {
		f.Usage()
		return client.ExitUsageError
	}

	status, errStatus := client.AssetsUpload(ctx, f.Arg(0), f.Arg(1))
	if errStatus != nil {
		return handleError(errStatus)
	}

	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	err := e.Encode(status)
	return handleError(err)
}

func assetsUploadCommand() subcommands.Command {
	return subcmd{
		assetsUploadCmd{},
		"upload",
		"upload asset",
		"upload NAME FILE",
	}
}

type assetsDeleteCmd struct{}

func (c assetsDeleteCmd) SetFlags(f *flag.FlagSet) {}

func (c assetsDeleteCmd) Execute(ctx context.Context, f *flag.FlagSet) subcommands.ExitStatus {
	if f.NArg() != 1 {
		f.Usage()
		return client.ExitUsageError
	}

	errStatus := client.AssetsDelete(ctx, f.Arg(0))
	return handleError(errStatus)
}

func assetsDeleteCommand() subcommands.Command {
	return subcmd{
		assetsDeleteCmd{},
		"delete",
		"delete asset",
		"delete NAME",
	}
}