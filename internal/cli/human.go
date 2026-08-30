package cli

import (
	"context"

	"github.com/DaikuFi/daiku-cli/internal/i18n"
	"github.com/spf13/cobra"
)

type humanContextKey struct{}

// HumanContext describes presentation capabilities for the current command.
// It is request-scoped so modules remain free of mutable process globals.
type HumanContext struct {
	Localizer   i18n.Localizer
	Terminal    bool
	Interactive bool
	Width       int
	NoColor     bool
	JSON        bool
}

func withHumanContext(ctx context.Context, human HumanContext) context.Context {
	return context.WithValue(ctx, humanContextKey{}, human)
}

// Human returns the presentation context installed by App. The English,
// non-interactive fallback keeps commands safe in isolated unit tests.
func Human(command *cobra.Command) HumanContext {
	if command != nil {
		if ctx := command.Context(); ctx != nil {
			if human, ok := ctx.Value(humanContextKey{}).(HumanContext); ok {
				return human
			}
		}
	}
	return HumanContext{Localizer: i18n.New(i18n.English)}
}
