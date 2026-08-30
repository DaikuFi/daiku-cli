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
	"skip the interactive confirmation":                          "omite la confirmación interactiva",
	"Added profile %s.\n":                                        "Perfil %s agregado.\n",
	"Using profile %s.\n":                                        "Usando el perfil %s.\n",
	"Removed profile %s.\n":                                      "Perfil %s eliminado.\n",
	"Logged in as profile %s.\n":                                 "Sesión iniciada con el perfil %s.\n",
	"Logged out profile %s.\n":                                   "Sesión cerrada para el perfil %s.\n",
	"Profile %s is logged in.\n":                                 "El perfil %s tiene una sesión activa.\n",
	"Profile %s is not logged in.\n":                             "El perfil %s no tiene una sesión activa.\n",
	"Open this URL to continue authentication:\n%s\n":            "Abre esta URL para continuar la autenticación:\n%s\n",
	"Remove profile %s.":                                         "Eliminar el perfil %s.",
	"Log out profile %s.":                                        "Cerrar la sesión del perfil %s.",
	"profile already exists":                                     "el perfil ya existe",
	"profile does not exist":                                     "el perfil no existe",
	"log out this profile before removing it":                    "cierra la sesión de este perfil antes de eliminarlo",
	"profile configuration could not be updated":                 "no se pudo actualizar la configuración de perfiles",
	"profile configuration could not be read":                    "no se pudo leer la configuración de perfiles",
	"select a profile before authenticating":                     "selecciona un perfil antes de autenticarte",
	"credentials could not be stored securely":                   "no se pudieron guardar las credenciales de forma segura",
	"credentials could not be read securely":                     "no se pudieron leer las credenciales de forma segura",
	"local credentials could not be removed":                     "no se pudieron eliminar las credenciales locales",
	"credentials could not be revoked; use --local-only to remove only the local copy": "no se pudieron revocar las credenciales; usa --local-only para eliminar sólo la copia local",
	"confirmation requires an interactive terminal; pass --yes to continue":            "la confirmación requiere una terminal interactiva; usa --yes para continuar",
	"operation cancelled":            "operación cancelada",
	"confirmation could not be read": "no se pudo leer la confirmación",
	"profile name must contain only letters, numbers, '.', '_' or '-'": "el nombre del perfil sólo puede contener letras, números, '.', '_' o '-'",
	"invalid API URL":                          "URL de API inválida",
	"API URL must be an absolute /api/v1/ URL": "la URL de API debe ser una URL absoluta que termine en /api/v1/",
	"API URL must use HTTPS":                   "la URL de API debe usar HTTPS",
	"NAME":                                     "NOMBRE",
	"API URL":                                  "URL DE API",
	"CURRENT":                                  "ACTUAL",
	"yes":                                      "sí",
	"no":                                       "no",
	"Manage households":                        "Gestiona hogares",
	"List households":                          "Lista los hogares",
	"Get a household":                          "Muestra un hogar",
	"Create a household":                       "Crea un hogar",
	"Update a household":                       "Actualiza un hogar",
	"Delete a household":                       "Elimina un hogar",
	"Reorder households":                       "Reordena los hogares",
	"Manage account groups":                    "Gestiona grupos de cuentas",
	"List account groups":                      "Lista los grupos de cuentas",
	"Create account group":                     "Crea un grupo de cuentas",
	"Update account group":                     "Actualiza un grupo de cuentas",
	"Delete account group":                     "Elimina un grupo de cuentas",
	"Reorder account groups":                   "Reordena los grupos de cuentas",
	"Manage accounts":                          "Gestiona cuentas",
	"List accounts":                            "Lista las cuentas",
	"Create an account":                        "Crea una cuenta",
	"Update an account":                        "Actualiza una cuenta",
	"Archive an account":                       "Archiva una cuenta",
	"Unarchive an account":                     "Desarchiva una cuenta",
	"Adjust an account balance":                "Ajusta el saldo de una cuenta",
	"Reorder accounts":                         "Reordena las cuentas",
	"Manage categories":                        "Gestiona categorías",
	"List categories":                          "Lista las categorías",
	"Create category":                          "Crea una categoría",
	"Update category":                          "Actualiza una categoría",
	"Delete category":                          "Elimina una categoría",
	"Reorder categories":                       "Reordena las categorías",
	"Manage tags":                              "Gestiona etiquetas",
	"List tags":                                "Lista las etiquetas",
	"Create tag":                               "Crea una etiqueta",
	"Update tag":                               "Actualiza una etiqueta",
	"Delete tag":                               "Elimina una etiqueta",
	"Manage institutions":                      "Gestiona instituciones",
	"List institutions":                        "Lista las instituciones",
	"Create institution":                       "Crea una institución",
	"Update institution":                       "Actualiza una institución",
	"Delete institution":                       "Elimina una institución",
	"household ID":                             "ID del hogar",
	"household name":                           "nombre del hogar",
	"display currency":                         "moneda de visualización",
	"household emoji":                          "emoji del hogar",
	"resource IDs in desired order":            "IDs de recursos en el orden deseado",
	"household IDs in desired order":           "IDs de hogares en el orden deseado",
	"account name":                             "nombre de la cuenta",
	"account type":                             "tipo de cuenta",
	"account group ID":                         "ID del grupo de cuentas",
	"institution ID":                           "ID de la institución",
	"opening balance":                          "saldo inicial",
	"target balance":                           "saldo objetivo",
	"adjustment date":                          "fecha del ajuste",
	"adjustment note":                          "nota del ajuste",
	"Delete household %s.":                     "Eliminar el hogar %s.",
	"Delete resource %s.":                      "Eliminar el recurso %s.",
	"Archive account %s.":                      "Archivar la cuenta %s.",
	"Adjust account %s balance.":               "Ajustar el saldo de la cuenta %s.",
	"clear the parent category":                "quita la categoría padre",
	"clear the account group":                  "quita el grupo de la cuenta",
	"clear the institution":                    "quita la institución",
	"name":                                     "nombre",
	"emoji":                                    "emoji",
	"color":                                    "color",
	"domain":                                   "dominio",
	"country":                                  "país",
	"currency":                                 "moneda",
	"ISO country code":                         "código ISO del país",
	"account number":                           "número de cuenta",
	"account holder":                           "titular de la cuenta",
	"make this the default account":            "establece esta cuenta como predeterminada",
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

// Resolve uses the explicit flag first, then the first present locale variable.
// A configured but unsupported value is an error instead of silently consulting
// a lower-priority variable, which keeps locale selection predictable.
func Resolve(explicit string, lookup func(string) (string, bool)) (Language, error) {
	if language, err := Parse(explicit); err != nil || language != "" {
		return language, err
	}
	for _, name := range []string{"DAIKU_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		value, ok := lookup(name)
		if !ok {
			continue
		}
		locale := strings.TrimSpace(strings.SplitN(value, ".", 2)[0])
		if strings.EqualFold(locale, "C") || strings.EqualFold(locale, "POSIX") {
			return English, nil
		}
		language, err := Parse(locale)
		if err != nil || language == "" {
			return "", fmt.Errorf("unsupported locale in %s: %q (use en or es)", name, value)
		}
		return language, nil
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

func (l Localizer) Humanf(format string, args ...any) string {
	return fmt.Sprintf(l.Human(format), args...)
}
