package cli

import "github.com/spf13/cobra"

// Module owns one command domain and registers its commands on the root.
// Keeping registration beside each domain lets feature branches evolve without
// editing a shared command implementation.
type Module interface {
	Register(root *cobra.Command)
}
