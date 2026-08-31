package version

import (
	"fmt"

	"github.com/DaikuFi/daiku-cli/internal/agent"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/spf13/cobra"
)

type Module struct {
	version string
}

func New(version string) Module {
	return Module{version: version}
}

func (module Module) Register(root *cobra.Command) {
	root.AddCommand(agent.ReadOnly(&cobra.Command{
		Use:   "version",
		Short: "Print the Daiku CLI version",
		Args:  cli.UsageArgs(cobra.NoArgs),
		RunE: func(command *cobra.Command, _ []string) error {
			jsonOutput, _ := command.Flags().GetBool("json")
			if jsonOutput {
				return cli.WriteSuccess(command.OutOrStdout(), map[string]string{"version": module.version})
			}
			_, err := fmt.Fprintln(command.OutOrStdout(), module.version)
			return err
		},
	}))
}
