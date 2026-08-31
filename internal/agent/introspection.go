// Package agent derives machine-readable CLI metadata directly from Cobra.
// It intentionally owns no command registry: the executable's Cobra tree is
// the single source of truth for both human help and agent introspection.
package agent

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const requiresInputAnnotation = "daiku.agent.requires_input"
const readOnlyAnnotation = "daiku.agent.read_only"

// Flag is the stable, machine-readable description of a command flag.
type Flag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default"`
	Usage     string `json:"usage"`
	Required  bool   `json:"required"`
	Inherited bool   `json:"inherited"`
}

// CommandSummary describes a direct child without recursively duplicating its
// complete metadata. Callers can resolve the path in List's flat command set.
type CommandSummary struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Short    string `json:"short"`
	Runnable bool   `json:"runnable"`
}

// Command is the complete discoverable contract for one Cobra command.
type Command struct {
	Name          string           `json:"name"`
	Path          string           `json:"path"`
	Use           string           `json:"use"`
	Short         string           `json:"short"`
	Long          string           `json:"long,omitempty"`
	Aliases       []string         `json:"aliases"`
	Runnable      bool             `json:"runnable"`
	Flags         []Flag           `json:"flags"`
	Subcommands   []CommandSummary `json:"subcommands"`
	ReadOnly      bool             `json:"read_only"`
	RequiresInput bool             `json:"requires_input"`
}

// Breadcrumb is a deterministic follow-up command an agent can execute.
type Breadcrumb struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// MarkRequiresInput marks a command that cannot safely run with --no-input.
func MarkRequiresInput(command *cobra.Command) {
	if command.Annotations == nil {
		command.Annotations = map[string]string{}
	}
	command.Annotations[requiresInputAnnotation] = "true"
}

// RequiresInput reports whether a command needs external or terminal input.
func RequiresInput(command *cobra.Command) bool {
	return command != nil && command.Annotations[requiresInputAnnotation] == "true"
}

// ReadOnly marks and returns a command whose handler has no side effects.
func ReadOnly(command *cobra.Command) *cobra.Command {
	if command.Annotations == nil {
		command.Annotations = map[string]string{}
	}
	command.Annotations[readOnlyAnnotation] = "true"
	return command
}

// Describe derives one command's public contract from Cobra metadata.
func Describe(command *cobra.Command) Command {
	command.InitDefaultHelpFlag()
	command.InitDefaultVersionFlag()

	description := Command{
		Name:          command.Name(),
		Path:          command.CommandPath(),
		Use:           strings.TrimSuffix(command.UseLine(), " [flags]"),
		Short:         command.Short,
		Long:          command.Long,
		Aliases:       append([]string(nil), command.Aliases...),
		Runnable:      command.Runnable(),
		Flags:         flags(command),
		Subcommands:   []CommandSummary{},
		ReadOnly:      isReadOnly(command),
		RequiresInput: RequiresInput(command),
	}
	if description.Aliases == nil {
		description.Aliases = []string{}
	}
	for _, child := range visibleChildren(command) {
		description.Subcommands = append(description.Subcommands, CommandSummary{
			Name: child.Name(), Path: child.CommandPath(), Short: child.Short, Runnable: child.Runnable(),
		})
	}
	return description
}

func isReadOnly(command *cobra.Command) bool {
	return command != nil && command.Annotations[readOnlyAnnotation] == "true"
}

// List returns every visible command exactly once, sorted by command path.
func List(root *cobra.Command) []Command {
	commands := make([]Command, 0)
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		commands = append(commands, Describe(command))
		for _, child := range visibleChildren(command) {
			walk(child)
		}
	}
	walk(root)
	sort.Slice(commands, func(i, j int) bool { return commands[i].Path < commands[j].Path })
	return commands
}

// Breadcrumbs returns generic discovery actions without inventing domain
// operations or relying on a second hand-maintained command catalog.
func Breadcrumbs(command *cobra.Command) []Breadcrumb {
	items := []Breadcrumb{{Command: "daiku commands --agent", Description: "List every available command and flag"}}
	if command == nil {
		return items
	}
	path := strings.TrimSpace(strings.TrimPrefix(command.CommandPath(), "daiku"))
	help := helpCommand(path)
	items = append(items, Breadcrumb{Command: help, Description: "Inspect structured help for this command"})
	if parent := command.Parent(); parent != nil {
		parentPath := strings.TrimSpace(strings.TrimPrefix(parent.CommandPath(), "daiku"))
		items = append(items, Breadcrumb{
			Command:     helpCommand(parentPath),
			Description: "Inspect the parent command",
		})
	}
	return items
}

func helpCommand(path string) string {
	if path == "" {
		return "daiku help --agent"
	}
	return "daiku help " + path + " --agent"
}

func visibleChildren(command *cobra.Command) []*cobra.Command {
	children := make([]*cobra.Command, 0)
	for _, child := range command.Commands() {
		if child.Hidden || (!child.IsAvailableCommand() && child.Name() != "help") {
			continue
		}
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
	return children
}

func flags(command *cobra.Command) []Flag {
	items := make([]Flag, 0)
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		required := false
		for _, annotation := range flag.Annotations[cobra.BashCompOneRequiredFlag] {
			required = required || annotation == "true"
		}
		items = append(items, Flag{
			Name:      flag.Name,
			Shorthand: flag.Shorthand,
			Type:      flag.Value.Type(),
			Default:   flag.DefValue,
			Usage:     flag.Usage,
			Required:  required,
			Inherited: command.InheritedFlags().Lookup(flag.Name) != nil,
		})
	})
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}
