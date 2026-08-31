package mcp

import (
	"context"
	"io"

	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/spf13/cobra"
)

type Runner func(context.Context, bool, io.ReadCloser, io.WriteCloser, io.Writer) error

type Module struct{ run Runner }

func New(run Runner) Module { return Module{run: run} }

func (m Module) Register(root *cobra.Command) {
	var allowWrite bool
	command := &cobra.Command{
		Use:   "mcp",
		Short: "Run the Daiku MCP server over stdio",
		Args:  cli.UsageArgs(cobra.NoArgs),
		RunE: func(command *cobra.Command, _ []string) error {
			in, ok := command.InOrStdin().(io.ReadCloser)
			if !ok {
				in = io.NopCloser(command.InOrStdin())
			}
			out, ok := command.OutOrStdout().(io.WriteCloser)
			if !ok {
				out = nopWriteCloser{command.OutOrStdout()}
			}
			return m.run(command.Context(), allowWrite, in, out, command.ErrOrStderr())
		},
	}
	command.Flags().BoolVar(&allowWrite, "allow-write", false, "allow confirmed write tools")
	root.AddCommand(command)
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
