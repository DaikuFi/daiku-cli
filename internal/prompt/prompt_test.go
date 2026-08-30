package prompt_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/internal/i18n"
	"github.com/DaikuFi/daiku-cli/internal/prompt"
)

func TestDestructiveConfirmationGoldenES(t *testing.T) {
	var output bytes.Buffer
	p := prompt.Prompter{In: strings.NewReader("sí\n"), Out: &output, Localize: i18n.New(i18n.Spanish), Terminal: true}
	if err := p.ConfirmDestructive("Eliminar hogar casa."); err != nil {
		t.Fatal(err)
	}
	want := "Eliminar hogar casa. ¿Continuar? Escribe sí para confirmar: "
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestDestructiveConfirmationRejectsPipeAndWeakAnswers(t *testing.T) {
	p := prompt.Prompter{In: strings.NewReader("yes\n"), Out: &bytes.Buffer{}, Localize: i18n.New(i18n.English)}
	if err := p.ConfirmDestructive("Delete household."); !errors.Is(err, prompt.ErrNonInteractive) {
		t.Fatalf("pipe error = %v", err)
	}
	p.Terminal = true
	p.In = strings.NewReader("y\n")
	if err := p.ConfirmDestructive("Delete household."); !errors.Is(err, prompt.ErrAborted) {
		t.Fatalf("weak answer error = %v", err)
	}
}

func TestAmbiguityGoldenENReturnsStableValue(t *testing.T) {
	var output bytes.Buffer
	p := prompt.Prompter{In: strings.NewReader("2\n"), Out: &output, Localize: i18n.New(i18n.English), Terminal: true}
	got, err := p.Select([]prompt.Choice{{Label: "Casa", Value: "hh_1"}, {Label: "Café", Value: "hh_2"}})
	if err != nil || got != "hh_2" {
		t.Fatalf("value=%q err=%v", got, err)
	}
	want := "Multiple matches found. Choose one:\n  1. Casa\n  2. Café\nEnter a number (1-2): "
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestAmbiguityNeverPromptsPipe(t *testing.T) {
	p := prompt.Prompter{In: strings.NewReader("1\n"), Out: &bytes.Buffer{}, Localize: i18n.New(i18n.English)}
	if _, err := p.Select([]prompt.Choice{{Value: "1"}, {Value: "2"}}); !errors.Is(err, prompt.ErrNonInteractive) {
		t.Fatalf("error = %v", err)
	}
}

func TestPrompterPreservesBufferedInputAcrossPrompts(t *testing.T) {
	var output bytes.Buffer
	p := prompt.Prompter{In: strings.NewReader("yes\n1\n"), Out: &output, Localize: i18n.New(i18n.English), Terminal: true}
	if err := p.ConfirmDestructive("Delete."); err != nil {
		t.Fatal(err)
	}
	value, err := p.Select([]prompt.Choice{{Label: "One", Value: "one"}, {Label: "Two", Value: "two"}})
	if err != nil || value != "one" {
		t.Fatalf("value=%q err=%v", value, err)
	}
}
