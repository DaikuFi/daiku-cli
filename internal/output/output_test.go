package output_test

import (
	"bytes"
	"testing"

	"github.com/DaikuFi/daiku-cli/internal/i18n"
	"github.com/DaikuFi/daiku-cli/internal/output"
)

var rows = []output.Row{
	{{Label: "NAME", Value: "café"}, {Label: "STATUS", Value: "active"}},
	{{Label: "NAME", Value: "ahorro"}, {Label: "STATUS", Value: "inactive"}},
}

func TestWideTableGolden(t *testing.T) {
	var buffer bytes.Buffer
	renderer := output.Renderer{Writer: &buffer, Localize: i18n.New(i18n.English), Width: 80}
	if err := renderer.Table(rows); err != nil {
		t.Fatal(err)
	}
	want := "NAME    STATUS  \n" +
		"café    active  \n" +
		"ahorro  inactive\n"
	if buffer.String() != want {
		t.Fatalf("output:\n%q\nwant:\n%q", buffer.String(), want)
	}
}

func TestNarrowTableGoldenAndUTF8(t *testing.T) {
	var buffer bytes.Buffer
	renderer := output.Renderer{Writer: &buffer, Localize: i18n.New(i18n.Spanish), Terminal: true, Width: 11, NoColor: true}
	if err := renderer.Table(rows[:1]); err != nil {
		t.Fatal(err)
	}
	want := "NAME: café\nSTATUS: active\n"
	if buffer.String() != want {
		t.Fatalf("output = %q, want %q", buffer.String(), want)
	}
}

func TestPipeAndNoColorNeverEmitANSI(t *testing.T) {
	for _, renderer := range []output.Renderer{
		{Terminal: false, NoColor: false},
		{Terminal: true, NoColor: true},
	} {
		var buffer bytes.Buffer
		renderer.Writer = &buffer
		renderer.Localize = i18n.New(i18n.English)
		renderer.Width = 8
		if err := renderer.Table(rows[:1]); err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(buffer.Bytes(), []byte("\x1b[")) {
			t.Fatalf("ANSI in %q", buffer.String())
		}
	}
}

func TestEmptyIsLocalized(t *testing.T) {
	var buffer bytes.Buffer
	renderer := output.Renderer{Writer: &buffer, Localize: i18n.New(i18n.Spanish)}
	if err := renderer.Table(nil); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); got != "No hay resultados.\n" {
		t.Fatalf("empty = %q", got)
	}
}
