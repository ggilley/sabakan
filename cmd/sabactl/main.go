package main

import (
	"context"
	"flag"
	"net/http"
	"os"

	"github.com/cybozu-go/cmd"
	"github.com/cybozu-go/log"
	"github.com/cybozu-go/sabakan/client"
	"github.com/google/subcommands"
)

var (
	flagServer = flag.String("server", "http://localhost:10080", "<Listen IP>:<Port number>")
)

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(dhcpCommand(), "")
	subcommands.Register(ipamCommand(), "")
	subcommands.Register(machinesCommand(), "")
	subcommands.Register(imagesCommand(), "")
	subcommands.Register(assetsCommand(), "")
	subcommands.Register(ignitionsCommand(), "")
	flag.Parse()
	cmd.LogConfig{}.Apply()

	client.Setup(*flagServer, &cmd.HTTPClient{
		Severity: log.LvDebug,
		Client:   &http.Client{},
	})

	exitStatus := subcommands.ExitSuccess
	cmd.Go(func(ctx context.Context) error {
		exitStatus = subcommands.Execute(ctx)
		return nil
	})
	cmd.Stop()
	cmd.Wait()
	os.Exit(int(exitStatus))
}
