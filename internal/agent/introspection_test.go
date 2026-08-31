package agent

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestListDerivesSortedTreeAndFlagsFromCobra(t *testing.T) {
	root := &cobra.Command{Use: "daiku", Short: "root"}
	root.PersistentFlags().Bool("json", false, "JSON output")
	create := &cobra.Command{Use: "create NAME", Short: "create item", RunE: func(*cobra.Command, []string) error { return nil }}
	create.Flags().StringP("household", "H", "", "household ID")
	_ = create.MarkFlagRequired("household")
	items := &cobra.Command{Use: "items", Short: "items"}
	items.AddCommand(create)
	root.AddCommand(items, &cobra.Command{Use: "version", Short: "version", Run: func(*cobra.Command, []string) {}})

	got := List(root)
	paths := make([]string, len(got))
	for index, command := range got {
		paths[index] = command.Path
	}
	want := []string{"daiku", "daiku items", "daiku items create", "daiku version"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths=%v want=%v", paths, want)
	}
	created := got[2]
	if !created.Runnable || created.Use != "daiku items create NAME" {
		t.Fatalf("create=%+v", created)
	}
	flags := map[string]Flag{}
	for _, flag := range created.Flags {
		flags[flag.Name] = flag
	}
	if !flags["household"].Required || flags["household"].Inherited || flags["household"].Shorthand != "H" {
		t.Fatalf("household=%+v", flags["household"])
	}
	if !flags["json"].Inherited {
		t.Fatalf("json=%+v", flags["json"])
	}
}

func TestRequiresInputAnnotation(t *testing.T) {
	command := &cobra.Command{Use: "login"}
	if RequiresInput(command) {
		t.Fatal("unmarked command requires input")
	}
	MarkRequiresInput(command)
	if !RequiresInput(command) {
		t.Fatal("marked command does not require input")
	}
}

func TestBreadcrumbsAreDeterministic(t *testing.T) {
	root := &cobra.Command{Use: "daiku"}
	child := &cobra.Command{Use: "transactions"}
	root.AddCommand(child)
	got := Breadcrumbs(child)
	if len(got) != 3 || got[0].Command != "daiku commands --agent" || got[1].Command != "daiku help transactions --agent" || got[2].Command != "daiku help --agent" {
		t.Fatalf("breadcrumbs=%+v", got)
	}
}
