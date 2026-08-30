// Package i18n contains the deliberately small human-language surface of the
// CLI. Machine-readable output, command names, flags, and error codes must not
// pass through this package.
package i18n

import (
	"fmt"
	"strings"
)

type Language string

const (
	English Language = "en"
	Spanish Language = "es"
)

type Key string

const (
	RootDescription Key = "root.description"
	UsageHeading    Key = "help.usage"
	CommandsHeading Key = "help.commands"
	FlagsHeading    Key = "help.flags"
	ErrorPrefix     Key = "error.prefix"
	NoResults       Key = "output.no_results"
	ConfirmPrompt   Key = "prompt.confirm"
	ConfirmHint     Key = "prompt.confirm_hint"
	AmbiguousPrompt Key = "prompt.ambiguous"
	AmbiguousHint   Key = "prompt.ambiguous_hint"
	InvalidChoice   Key = "prompt.invalid_choice"
	Aborted         Key = "prompt.aborted"
)

var messages = map[Language]map[Key]string{
	English: {
		RootDescription: "Manage Daiku from the command line",
		UsageHeading:    "Usage",
		CommandsHeading: "Commands",
		FlagsHeading:    "Flags",
		ErrorPrefix:     "Error",
		NoResults:       "No results.",
		ConfirmPrompt:   "%s Continue?",
		ConfirmHint:     "Type %s to confirm",
		AmbiguousPrompt: "Multiple matches found. Choose one:",
		AmbiguousHint:   "Enter a number (1-%d)",
		InvalidChoice:   "Invalid choice.",
		Aborted:         "Cancelled.",
	},
	Spanish: {
		RootDescription: "Gestiona Daiku desde la línea de comandos",
		UsageHeading:    "Uso",
		CommandsHeading: "Comandos",
		FlagsHeading:    "Opciones",
		ErrorPrefix:     "Error",
		NoResults:       "No hay resultados.",
		ConfirmPrompt:   "%s ¿Continuar?",
		ConfirmHint:     "Escribe %s para confirmar",
		AmbiguousPrompt: "Se encontraron varias coincidencias. Elige una:",
		AmbiguousHint:   "Ingresa un número (1-%d)",
		InvalidChoice:   "Opción inválida.",
		Aborted:         "Cancelado.",
	},
}

var humanSpanish = map[string]string{
	"Manage Daiku from the command line":                         "Gestiona Daiku desde la línea de comandos",
	"Help about any command":                                     "Ayuda sobre cualquier comando",
	"Help provides help for any command in the application.":     "Muestra ayuda sobre cualquier comando de la aplicación.",
	"Print the Daiku CLI version":                                "Muestra la versión del CLI de Daiku",
	"Manage named Daiku profiles":                                "Gestiona perfiles de Daiku",
	"Add a profile":                                              "Agrega un perfil",
	"Select the active profile":                                  "Selecciona el perfil activo",
	"List profiles":                                              "Lista los perfiles",
	"Remove a profile and its local credentials":                 "Elimina un perfil y sus credenciales locales",
	"Authenticate with Daiku":                                    "Autentícate con Daiku",
	"Sign in using OAuth":                                        "Inicia sesión con OAuth",
	"Revoke and remove credentials":                              "Revoca y elimina las credenciales",
	"Show authentication status":                                 "Muestra el estado de autenticación",
	"Generate the autocompletion script for the specified shell": "Genera el script de autocompletado para el shell indicado",
	"write a stable JSON envelope":                               "escribe un envelope JSON estable",
	"human output language: en or es":                            "idioma de salida humana: en o es",
	"Daiku API URL":                                              "URL de la API de Daiku",
	"remove local credentials without revoking the token":        "elimina credenciales locales sin revocar el token",
}

// Parse accepts only the two product languages. Empty means auto-detect.
func Parse(value string) (Language, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	value = strings.SplitN(strings.ReplaceAll(value, "_", "-"), "-", 2)[0]
	switch Language(value) {
	case English, Spanish:
		return Language(value), nil
	default:
		return "", fmt.Errorf("unsupported language %q (use en or es)", value)
	}
}

// Resolve uses the explicit flag first, then DAIKU_LANG, then the conventional
// locale variables. Unknown environment locales safely fall back to English.
func Resolve(explicit string, lookup func(string) (string, bool)) (Language, error) {
	if language, err := Parse(explicit); err != nil || language != "" {
		return language, err
	}
	for _, name := range []string{"DAIKU_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		value, ok := lookup(name)
		if !ok || value == "" {
			continue
		}
		language, err := Parse(strings.SplitN(value, ".", 2)[0])
		if err == nil && language != "" {
			return language, nil
		}
	}
	return English, nil
}

type Localizer struct{ Language Language }

func New(language Language) Localizer {
	if language != Spanish {
		language = English
	}
	return Localizer{Language: language}
}

func (l Localizer) Text(key Key, args ...any) string {
	message, ok := messages[l.Language][key]
	if !ok {
		message = messages[English][key]
	}
	return fmt.Sprintf(message, args...)
}

// Human translates prose only. Identifiers embedded in Cobra Use strings are
// intentionally left untouched so commands and flags remain English.
func (l Localizer) Human(value string) string {
	if l.Language == Spanish {
		if strings.HasPrefix(value, "help for ") {
			return "ayuda de " + strings.TrimPrefix(value, "help for ")
		}
		if translated, ok := humanSpanish[value]; ok {
			return translated
		}
	}
	return value
}
