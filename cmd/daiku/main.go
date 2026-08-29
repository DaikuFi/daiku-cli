package main

import (
	"os"

	"github.com/DaikuFi/daiku-cli/internal/cli"
	versioncommand "github.com/DaikuFi/daiku-cli/internal/commands/version"
)

var version = "dev"

func main() {
	app := cli.New(
		cli.WithVersion(version),
		cli.WithModule(versioncommand.New(version)),
	)
	os.Exit(app.Run(os.Args[1:]))
}
