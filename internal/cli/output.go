package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/DaikuFi/daiku-cli/internal/i18n"
)

type successEnvelope struct {
	OK   bool `json:"ok"`
	Data any  `json:"data"`
}

type errorEnvelope struct {
	OK    bool         `json:"ok"`
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

// WriteSuccess emits the stable JSON success envelope.
func WriteSuccess(w io.Writer, data any) error {
	return writeJSON(w, successEnvelope{OK: true, Data: data})
}

func writeError(w io.Writer, err *Error, jsonOutput bool, localizer i18n.Localizer) {
	if jsonOutput {
		_ = writeJSON(w, errorEnvelope{
			OK:    false,
			Error: errorPayload{Code: err.Code, Message: err.Message, Details: err.Details},
		})
		return
	}

	_, _ = fmt.Fprintf(w, "%s: %s\n", localizer.Text(i18n.ErrorPrefix), err.Message)
}
