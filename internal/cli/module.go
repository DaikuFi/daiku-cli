package cli

import "github.com/spf13/cobra"

// Module owns one command domain and registers its commands on the root.
// Keeping registration beside each domain lets feature branches evolve without
// editing a shared command implementation.
type Module interface {
	Register(root *cobra.Command)
}

// UsageArgs marks errors from a trusted Cobra positional-argument validator as
// user input errors. Domain RunE errors must not be wrapped with this helper.
func UsageArgs(validator cobra.PositionalArgs) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if err := validator(command, args); err != nil {
			return usageError(err.Error())
		}
		return nil
	}
}
