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
	"invalid API URL":                            "URL de API inválida",
	"API URL must be an absolute /api/v1/ URL":   "la URL de API debe ser una URL absoluta que termine en /api/v1/",
	"API URL must use HTTPS":                     "la URL de API debe usar HTTPS",
	"Manage transactions":                        "Gestiona transacciones",
	"Manage transfers":                           "Gestiona transferencias",
	"Manage installment plans":                   "Gestiona planes de cuotas",
	"List transactions":                          "Lista transacciones",
	"Search transactions":                        "Busca transacciones",
	"Create a transaction":                       "Crea una transacción",
	"Update a transaction":                       "Actualiza una transacción",
	"Delete a transaction":                       "Elimina una transacción",
	"Create transactions in bulk":                "Crea transacciones en lote",
	"Update matching transactions in bulk":       "Actualiza transacciones en lote",
	"Delete all transactions":                    "Elimina todas las transacciones",
	"Create a balanced transfer":                 "Crea una transferencia balanceada",
	"Convert a transaction to a transfer":        "Convierte una transacción en transferencia",
	"List transfer candidates":                   "Lista candidatos para una transferencia",
	"Unlink both transfer legs":                  "Desvincula ambas partes de una transferencia",
	"Create an installment plan":                 "Crea un plan de cuotas",
	"Show an installment plan":                   "Muestra un plan de cuotas",
	"Update an installment plan":                 "Actualiza un plan de cuotas",
	"household ID":                               "ID del hogar",
	"search query":                               "texto de búsqueda",
	"inclusive start date":                       "fecha inicial inclusiva",
	"inclusive end date":                         "fecha final inclusiva",
	"account ID":                                 "ID de la cuenta",
	"category ID":                                "ID de la categoría",
	"recurring or one-time":                      "recurrente o única",
	"expense or income":                          "gasto o ingreso",
	"newest, oldest, amount_high, or amount_low": "newest, oldest, amount_high o amount_low",
	"tag ID (repeatable)":                        "ID de etiqueta (repetible)",
	"decimal amount":                             "importe decimal",
	"description":                                "descripción",
	"amount posted to the selected account":      "importe registrado en la cuenta seleccionada",
	"currency code published by the transaction API contract": "código de moneda publicado por el contrato de transacciones",
	"transaction date (YYYY-MM-DD)":                           "fecha de la transacción (AAAA-MM-DD)",
	"recurring expense ID":                                    "ID de la transacción recurrente",
	"expense, income, transfer, or adjustment":                "gasto, ingreso, transferencia o ajuste",
	"set account to null":                                     "establece la cuenta en null",
	"set account_amount to null":                              "establece account_amount en null",
	"set category to null":                                    "establece la categoría en null",
	"set recurring_expense to null":                           "establece recurring_expense en null",
	"replace tag_ids with an empty list":                      "reemplaza tag_ids por una lista vacía",
	"this, future, or plan for installments":                  "this, future o plan para cuotas",
	"JSON file, or - for stdin":                               "archivo JSON, o - para stdin",
	"source account ID":                                       "ID de la cuenta de origen",
	"destination account ID":                                  "ID de la cuenta de destino",
	"source decimal amount":                                   "importe decimal de origen",
	"destination decimal amount":                              "importe decimal de destino",
	"transfer date":                                           "fecha de la transferencia",
	"existing peer transaction ID":                            "ID de la transacción contraparte existente",
	"peer decimal amount":                                     "importe decimal de la contraparte",
	"purchase total as decimal string":                        "total de la compra como importe decimal",
	"currency code published by the installment API contract": "código de moneda publicado por el contrato de cuotas",
	"purchase date":                                           "fecha de compra",
	"number of installments":                                  "cantidad de cuotas",
	"purchase total, never cuota amount":                      "total de la compra, nunca el importe de una cuota",
	"UYU, USD, or EUR":                                        "UYU, USD o EUR",
	"transaction service is not configured":                   "el servicio de transacciones no está configurado",
	"date must use YYYY-MM-DD":                                "la fecha debe usar AAAA-MM-DD",
	"amount must be a positive decimal string with at most two fractional digits":      "el importe debe ser un decimal positivo con hasta dos dígitos fraccionarios",
	"amount must be a decimal string with at most ten whole and two fractional digits": "el importe debe ser un decimal con hasta diez dígitos enteros y dos fraccionarios",
	"currency is not supported by the transaction API contract":                        "la moneda no está admitida por el contrato de transacciones",
	"currency is not supported by the installment API contract":                        "la moneda no está admitida por el contrato de cuotas",
	"select a profile before using transactions":                                       "selecciona un perfil antes de usar transacciones",
	"authentication manager is not configured":                                         "el gestor de autenticación no está configurado",
	"authenticate this profile before using transactions":                              "autentica este perfil antes de usar transacciones",
	"profile configuration contains an invalid API URL":                                "la configuración del perfil contiene una URL de API inválida",
	"the Daiku API client could not be created":                                        "no se pudo crear el cliente de la API de Daiku",
	"the Daiku API returned an invalid transaction list":                               "la API de Daiku devolvió una lista de transacciones inválida",
	"transaction update could not be encoded":                                          "no se pudo codificar la actualización de la transacción",
	"bulk update could not be encoded":                                                 "no se pudo codificar la actualización en lote",
	"installment update could not be encoded":                                          "no se pudo codificar la actualización del plan de cuotas",
	"--query is required":                                                              "--query es obligatorio",
	"--from must not be after --to":                                                    "--from no puede ser posterior a --to",
	"kind must be recurring or one-time":                                               "kind debe ser recurring o one-time",
	"type must be expense or income":                                                   "type debe ser expense o income",
	"invalid ordering":                                                                 "orden inválido",
	"a value flag cannot be combined with its clear flag":                              "una opción con valor no puede combinarse con su opción para limpiar",
	"invalid transaction type":                                                         "tipo de transacción inválido",
	"at least one update flag is required":                                             "se requiere al menos una opción de actualización",
	"scope must be this, future, or plan":                                              "scope debe ser this, future o plan",
	"input file could not be opened":                                                   "no se pudo abrir el archivo de entrada",
	"input must be valid JSON matching the API contract":                               "la entrada debe ser JSON válido y coincidir con el contrato de la API",
	"input must contain exactly one JSON value":                                        "la entrada debe contener exactamente un valor JSON",
	"expenses must not be empty":                                                       "expenses no puede estar vacío",
	"ids and updates are required":                                                     "ids y updates son obligatorios",
	"source and destination accounts must differ":                                      "las cuentas de origen y destino deben ser distintas",
	"exactly one of --to-account or --peer is required":                                "se requiere exactamente una de --to-account o --peer",
	"count must be between 2 and 60":                                                   "count debe estar entre 2 y 60",
	"amount must be at least 0.01 per installment":                                     "el importe debe ser al menos 0,01 por cuota",
	"NAME":    "NOMBRE",
	"API URL": "URL DE API",
	"CURRENT": "ACTUAL",
	"yes":     "sí",
	"no":      "no",
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
		if suffix, ok := strings.CutPrefix(value, "Daiku API returned HTTP "); ok {
			return "La API de Daiku respondió HTTP " + suffix
		}
	}
	return value
}

func (l Localizer) Humanf(format string, args ...any) string {
	return fmt.Sprintf(l.Human(format), args...)
}
