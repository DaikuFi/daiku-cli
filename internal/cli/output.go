package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/DaikuFi/daiku-cli/internal/agent"
	"github.com/DaikuFi/daiku-cli/internal/i18n"
)

type successEnvelope struct {
	OK          bool               `json:"ok"`
	Data        any                `json:"data"`
	Breadcrumbs []agent.Breadcrumb `json:"breadcrumbs,omitempty"`
}

type errorEnvelope struct {
	OK          bool               `json:"ok"`
	Error       errorPayload       `json:"error"`
	Breadcrumbs []agent.Breadcrumb `json:"breadcrumbs,omitempty"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type agentSuccessEnvelope struct {
	OK          bool               `json:"ok"`
	Data        json.RawMessage    `json:"data"`
	Breadcrumbs []agent.Breadcrumb `json:"breadcrumbs,omitempty"`
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

func writeAgentSuccess(w io.Writer, raw []byte, breadcrumbs []agent.Breadcrumb) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return errors.New("agent command returned no JSON output")
	}
	var envelope agentSuccessEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil || !envelope.OK {
		return errors.New("agent command returned an invalid JSON envelope")
	}
	if len(envelope.Data) == 0 {
		return errors.New("agent command returned an invalid JSON envelope")
	}
	if len(envelope.Breadcrumbs) == 0 {
		envelope.Breadcrumbs = breadcrumbs
	}
	return writeJSON(w, envelope)
}

func writeError(w io.Writer, err *Error, jsonOutput bool, localizer i18n.Localizer, breadcrumbs ...agent.Breadcrumb) {
	if jsonOutput {
		_ = writeJSON(w, errorEnvelope{
			OK:          false,
			Error:       errorPayload{Code: err.Code, Message: err.Message, Details: err.Details},
			Breadcrumbs: breadcrumbs,
		})
		return
	}

	_, _ = fmt.Fprintf(w, "%s: %s\n", localizer.Text(i18n.ErrorPrefix), localizer.Human(err.Message))
}
