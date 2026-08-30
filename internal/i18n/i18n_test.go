package i18n_test

import (
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/internal/i18n"
)

func TestResolvePrecedenceAndLocaleVariants(t *testing.T) {
	environment := map[string]string{"DAIKU_LANG": "es-UY", "LANG": "en_US.UTF-8"}
	lookup := func(key string) (string, bool) { value, ok := environment[key]; return value, ok }

	if got, err := i18n.Resolve("en", lookup); err != nil || got != i18n.English {
		t.Fatalf("explicit language: got %q err=%v", got, err)
	}
	if got, err := i18n.Resolve("", lookup); err != nil || got != i18n.Spanish {
		t.Fatalf("environment language: got %q err=%v", got, err)
	}
}

func TestResolveDefaultsToEnglishAndExplicitInvalidFails(t *testing.T) {
	lookup := func(string) (string, bool) { return "", false }
	if got, err := i18n.Resolve("", lookup); err != nil || got != i18n.English {
		t.Fatalf("fallback: got %q err=%v", got, err)
	}
	if _, err := i18n.Resolve("pt", lookup); err == nil {
		t.Fatal("expected invalid explicit language to fail")
	}
}

func TestResolveFirstPresentVariableWinsEvenWhenInvalid(t *testing.T) {
	environment := map[string]string{"LC_ALL": "pt_BR.UTF-8", "LANG": "es_UY.UTF-8"}
	lookup := func(key string) (string, bool) { value, ok := environment[key]; return value, ok }
	if _, err := i18n.Resolve("", lookup); err == nil || !strings.Contains(err.Error(), "LC_ALL") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveCLocalesAsEnglish(t *testing.T) {
	for _, value := range []string{"C", "C.UTF-8", "POSIX"} {
		lookup := func(string) (string, bool) { return value, true }
		if got, err := i18n.Resolve("", lookup); err != nil || got != i18n.English {
			t.Fatalf("locale=%q got=%q err=%v", value, got, err)
		}
	}
}

func TestLocalizerHandlesUTF8(t *testing.T) {
	got := i18n.New(i18n.Spanish).Text(i18n.RootDescription)
	if got != "Gestiona Daiku desde la línea de comandos" {
		t.Fatalf("translation = %q", got)
	}
}
