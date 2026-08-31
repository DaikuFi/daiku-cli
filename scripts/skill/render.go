//go:build ignore

// Command render converts the live agent command envelope into checked-in skill references.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type envelope struct {
	OK   bool `json:"ok"`
	Data struct {
		Commands []command `json:"commands"`
	} `json:"data"`
}

type command struct {
	Name        string           `json:"name"`
	Path        string           `json:"path"`
	Use         string           `json:"use"`
	Short       string           `json:"short"`
	Long        string           `json:"long,omitempty"`
	Aliases     []string         `json:"aliases"`
	Runnable    bool             `json:"runnable"`
	Flags       []flag           `json:"flags"`
	Subcommands []commandSummary `json:"subcommands"`
}

type commandSummary struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Short    string `json:"short"`
	Runnable bool   `json:"runnable"`
}

type flag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default"`
	Usage     string `json:"usage"`
	Required  bool   `json:"required"`
	Inherited bool   `json:"inherited"`
}

func main() {
	if len(os.Args) != 4 {
		fatalf("usage: render INPUT_JSON OUTPUT_JSON OUTPUT_MARKDOWN")
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fatalf("read command envelope: %v", err)
	}
	var source envelope
	if err := json.Unmarshal(raw, &source); err != nil || !source.OK {
		fatalf("decode successful command envelope: %v", err)
	}
	sort.Slice(source.Data.Commands, func(i, j int) bool { return source.Data.Commands[i].Path < source.Data.Commands[j].Path })

	manifest, err := json.MarshalIndent(struct {
		Commands []command `json:"commands"`
	}{Commands: source.Data.Commands}, "", "  ")
	if err != nil {
		fatalf("encode command manifest: %v", err)
	}
	manifest = append(manifest, '\n')
	if err := os.WriteFile(os.Args[2], manifest, 0o644); err != nil {
		fatalf("write command manifest: %v", err)
	}

	var markdown strings.Builder
	markdown.WriteString("# Daiku command reference\n\nGenerated from `daiku commands --agent`; do not edit by hand. Live introspection wins if this reference differs.\n\n")
	for _, item := range source.Data.Commands {
		if !item.Runnable {
			continue
		}
		fmt.Fprintf(&markdown, "## `%s`\n\n%s\n\nUsage: `%s`\n", item.Path, item.Short, item.Use)
		local := make([]flag, 0)
		for _, value := range item.Flags {
			if !value.Inherited && value.Name != "help" {
				local = append(local, value)
			}
		}
		if len(local) > 0 {
			markdown.WriteString("\nFlags:\n\n")
			for _, value := range local {
				required := ""
				if value.Required {
					required = " (required)"
				}
				fmt.Fprintf(&markdown, "- `--%s` (%s)%s: %s\n", value.Name, value.Type, required, value.Usage)
			}
		}
		markdown.WriteByte('\n')
	}
	reference := strings.TrimRight(markdown.String(), "\n") + "\n"
	if err := os.WriteFile(os.Args[3], []byte(reference), 0o644); err != nil {
		fatalf("write command reference: %v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "skill reference generator: "+format+"\n", args...)
	os.Exit(1)
}
