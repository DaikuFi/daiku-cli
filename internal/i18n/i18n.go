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
	"Inspect budget summaries and manage budget rules":    "Consulta resúmenes y gestiona reglas de presupuesto",
	"Show a monthly budget summary":                       "Muestra un resumen mensual del presupuesto",
	"Show planned budgets for a year":                     "Muestra los presupuestos planificados para un año",
	"Show budget suggestions for a month":                 "Muestra sugerencias de presupuesto para un mes",
	"Manage category budget rules":                        "Gestiona reglas de presupuesto por categoría",
	"List category budget rules":                          "Lista las reglas de presupuesto por categoría",
	"Create a category budget rule":                       "Crea una regla de presupuesto por categoría",
	"Update a category budget rule":                       "Actualiza una regla de presupuesto por categoría",
	"Delete a category budget rule":                       "Elimina una regla de presupuesto por categoría",
	"Manage recurring templates and their occurrences":    "Gestiona plantillas recurrentes y sus ocurrencias",
	"List recurring templates":                            "Lista las plantillas recurrentes",
	"Create a recurring template":                         "Crea una plantilla recurrente",
	"Update a recurring template":                         "Actualiza una plantilla recurrente",
	"Delete a recurring template":                         "Elimina una plantilla recurrente",
	"List and resolve server-generated occurrences":       "Lista y resuelve ocurrencias generadas por el servidor",
	"List recurring occurrences":                          "Lista las ocurrencias recurrentes",
	"Confirm an occurrence as a human entry":              "Confirma una ocurrencia como entrada humana",
	"Skip a pending occurrence":                           "Omite una ocurrencia pendiente",
	"Snooze a pending occurrence":                         "Pospone una ocurrencia pendiente",
	"display currency: UYU, USD or EUR":                   "moneda de visualización: UYU, USD o EUR",
	"calendar year":                                       "año calendario",
	"month (1-12)":                                        "mes (1-12)",
	"category ID":                                         "ID de categoría",
	"budget amount":                                       "monto del presupuesto",
	"scope: monthly, yearly or month":                     "alcance: monthly, yearly o month",
	"pinned year (required for month scope)":              "año fijado (obligatorio para alcance month)",
	"clear the pinned month":                              "elimina el mes fijado",
	"clear the pinned year":                               "elimina el año fijado",
	"ISO currency published by the API contract":          "moneda ISO publicada por el contrato de API",
	"frequency: monthly or yearly":                        "frecuencia: monthly o yearly",
	"transaction type: expense, income or transfer":       "tipo de movimiento: expense, income o transfer",
	"creation mode: auto or confirm":                      "modo de creación: auto o confirm",
	"source account ID":                                   "ID de cuenta de origen",
	"destination account ID for transfers":                "ID de cuenta de destino para transferencias",
	"start date (YYYY-MM-DD)":                             "fecha de inicio (AAAA-MM-DD)",
	"day of month (1-31)":                                 "día del mes (1-31)",
	"month of year for yearly templates (1-12)":           "mes del año para plantillas anuales (1-12)",
	"whether the template is active":                      "si la plantilla está activa",
	"clear the source account":                            "elimina la cuenta de origen",
	"clear the destination account":                       "elimina la cuenta de destino",
	"clear the category":                                  "elimina la categoría",
	"clear the month of year":                             "elimina el mes del año",
	"occurrence status":                                   "estado de la ocurrencia",
	"final date (YYYY-MM-DD)":                             "fecha final (AAAA-MM-DD)",
	"final amount":                                        "monto final",
	"final destination amount for transfers":              "monto final de destino para transferencias",
	"reminder date (YYYY-MM-DD)":                          "fecha del recordatorio (AAAA-MM-DD)",
	"Budget summary for %02d/%d (%s).\n":                  "Resumen del presupuesto para %02d/%d (%s).\n",
	"Planned budgets for %d (%s).\n":                      "Presupuestos planificados para %d (%s).\n",
	"Budget suggestions for %02d/%d (%s).\n":              "Sugerencias de presupuesto para %02d/%d (%s).\n",
	"%d budget rules.\n":                                  "%d reglas de presupuesto.\n",
	"Budget rule created.\n":                              "Regla de presupuesto creada.\n",
	"Budget rule updated.\n":                              "Regla de presupuesto actualizada.\n",
	"Budget rule deleted.\n":                              "Regla de presupuesto eliminada.\n",
	"Delete budget rule %s.":                              "Eliminar la regla de presupuesto %s.",
	"%d recurring templates.\n":                           "%d plantillas recurrentes.\n",
	"Recurring template created.\n":                       "Plantilla recurrente creada.\n",
	"Recurring template updated.\n":                       "Plantilla recurrente actualizada.\n",
	"Recurring template deleted.\n":                       "Plantilla recurrente eliminada.\n",
	"Delete recurring template %s.":                       "Eliminar la plantilla recurrente %s.",
	"%d recurring occurrences.\n":                         "%d ocurrencias recurrentes.\n",
	"Recurring occurrence confirmed.\n":                   "Ocurrencia recurrente confirmada.\n",
	"Recurring occurrence skipped.\n":                     "Ocurrencia recurrente omitida.\n",
	"Recurring occurrence snoozed.\n":                     "Ocurrencia recurrente pospuesta.\n",
	"Confirm recurring occurrence %s.":                    "Confirmar la ocurrencia recurrente %s.",
	"Skip recurring occurrence %s.":                       "Omitir la ocurrencia recurrente %s.",
	"currency is not supported by the Daiku API contract": "la moneda no está publicada por el contrato de API de Daiku",
	"provide at least one field to update":                "indica al menos un campo para actualizar",
	"a field cannot be set and cleared together":          "un campo no se puede establecer y eliminar a la vez",
	"your role does not allow this operation":             "tu rol no permite esta operación",
	"the occurrence is already resolved":                  "la ocurrencia ya está resuelta",
	"description":                                         "descripción",
	"amount":                                              "monto",
	"currency: UYU, USD or EUR":                           "moneda: UYU, USD o EUR",
	"month (required for month scope)":                    "mes (obligatorio para alcance month)",
	"month must be between 1 and 12":                      "el mes debe estar entre 1 y 12",
	"year must be provided and greater than zero":         "debes indicar un año mayor que cero",
	"currency must be UYU, USD or EUR":                    "la moneda debe ser UYU, USD o EUR",
	"category, amount, currency and scope are required":   "categoría, monto, moneda y alcance son obligatorios",
	"scope must be monthly, yearly or month":              "el alcance debe ser monthly, yearly o month",
	"month scope requires --month 1-12 and --year":        "el alcance month requiere --month 1-12 y --year",
	"month and year are only valid for month scope":       "mes y año solo son válidos para el alcance month",
	"description, amount, currency, frequency, type, creation-mode and day are required": "descripción, monto, moneda, frecuencia, tipo, modo de creación y día son obligatorios",
	"frequency must be monthly or yearly":                                                "la frecuencia debe ser monthly o yearly",
	"type must be expense, income or transfer":                                           "el tipo debe ser expense, income o transfer",
	"creation-mode must be auto or confirm":                                              "el modo de creación debe ser auto o confirm",
	"day must be between 1 and 31":                                                       "el día debe estar entre 1 y 31",
	"yearly frequency requires --month":                                                  "la frecuencia yearly requiere --month",
	"month is only valid for yearly frequency":                                           "el mes solo es válido para la frecuencia yearly",
	"transfer templates require --account and --destination-account":                     "las plantillas de transferencia requieren --account y --destination-account",
	"destination-account is only valid for transfers":                                    "destination-account solo es válido para transferencias",
	"status must be all, pending, confirmed, skipped or superseded":                      "el estado debe ser all, pending, confirmed, skipped o superseded",
	"amount is required":                                                                 "el monto es obligatorio",
	"date must use YYYY-MM-DD":                                                           "la fecha debe usar AAAA-MM-DD",
	"authentication is required":                                                         "se requiere autenticación",
	"the requested resource was not found":                                               "no se encontró el recurso solicitado",
	"the Daiku API request failed":                                                       "falló la solicitud a la API de Daiku",
	"select and authenticate a profile first":                                            "selecciona y autentica un perfil primero",
	"the active profile is not authenticated":                                            "el perfil activo no está autenticado",
	"Manage portfolios and inspect server-calculated totals":                            "Gestiona portafolios y consulta totales calculados por el servidor",
	"List portfolios":                                                                    "Lista portafolios",
	"Get a portfolio":                                                                    "Muestra un portafolio",
	"Create a portfolio":                                                                 "Crea un portafolio",
	"Update a portfolio":                                                                 "Actualiza un portafolio",
	"Delete a portfolio":                                                                 "Elimina un portafolio",
	"Show server-calculated assets, liabilities and net worth":                           "Muestra activos, pasivos y patrimonio calculados por el servidor",
	"Show server-calculated portfolio holdings":                                         "Muestra las tenencias calculadas por el servidor",
	"Manage portfolio buckets":                                                           "Gestiona grupos del portafolio",
	"List buckets":                                                                       "Lista grupos",
	"Create a bucket":                                                                    "Crea un grupo",
	"Update a bucket":                                                                    "Actualiza un grupo",
	"Delete a bucket":                                                                    "Elimina un grupo",
	"Manage assets, cashflows and value history":                                         "Gestiona activos, flujos de caja e historial de valor",
	"List assets":                                                                        "Lista activos",
	"Create an asset":                                                                    "Crea un activo",
	"Update an asset":                                                                    "Actualiza un activo",
	"Delete an asset":                                                                    "Elimina un activo",
	"Manage asset cashflows":                                                             "Gestiona flujos de caja del activo",
	"List cashflows, including transaction links":                                        "Lista flujos de caja, incluidos sus vínculos a transacciones",
	"Create a cashflow":                                                                  "Crea un flujo de caja",
	"Update a cashflow":                                                                  "Actualiza un flujo de caja",
	"Delete a cashflow":                                                                  "Elimina un flujo de caja",
	"Manage asset value history":                                                         "Gestiona el historial de valor del activo",
	"List value history":                                                                 "Lista el historial de valor",
	"Create a value-history point":                                                       "Crea un punto del historial de valor",
	"Update a value-history point":                                                       "Actualiza un punto del historial de valor",
	"Delete a value-history point":                                                       "Elimina un punto del historial de valor",
	"portfolio name":                                                                     "nombre del portafolio",
	"portfolio emoji":                                                                    "emoji del portafolio",
	"portfolio ID":                                                                       "ID del portafolio",
	"make this the default portfolio":                                                    "lo establece como portafolio predeterminado",
	"bucket name":                                                                        "nombre del grupo",
	"bucket type":                                                                        "tipo de grupo",
	"bucket emoji":                                                                       "emoji del grupo",
	"bucket ID":                                                                          "ID del grupo",
	"sort order":                                                                         "orden",
	"asset name":                                                                         "nombre del activo",
	"asset type":                                                                         "tipo de activo",
	"asset ID":                                                                           "ID del activo",
	"current value (sent verbatim to Daiku)":                                             "valor actual (enviado sin cambios a Daiku)",
	"quantity":                                                                           "cantidad",
	"price per unit":                                                                     "precio por unidad",
	"ticker symbol":                                                                      "símbolo bursátil",
	"notes":                                                                              "notas",
	"mark as liability":                                                                  "lo marca como pasivo",
	"exclude from projections":                                                           "lo excluye de las proyecciones",
	"date (YYYY-MM-DD)":                                                                  "fecha (AAAA-MM-DD)",
	"cash in amount":                                                                     "monto de entrada",
	"cash out amount":                                                                    "monto de salida",
	"cash in currency":                                                                   "moneda de entrada",
	"cash out currency":                                                                  "moneda de salida",
	"recorded value":                                                                     "valor registrado",
	"recorded quantity":                                                                  "cantidad registrada",
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
