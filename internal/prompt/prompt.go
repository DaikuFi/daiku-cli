// Package prompt provides guarded interaction for human terminals. Commands
// must offer explicit flags for automation; this package never prompts a pipe.
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/DaikuFi/daiku-cli/internal/i18n"
)

var (
	ErrNonInteractive = errors.New("confirmation requires an interactive terminal")
	ErrAborted        = errors.New("operation cancelled")
)

type Prompter struct {
	In       io.Reader
	Out      io.Writer
	Localize i18n.Localizer
	Terminal bool
	reader   *bufio.Reader
}

// ConfirmDestructive requires the localized full confirmation word. A blank,
// yes/no abbreviation, EOF, or mismatched word cancels safely.
func (p *Prompter) ConfirmDestructive(action string) error {
	if !p.Terminal {
		return ErrNonInteractive
	}
	word := "yes"
	if p.Localize.Language == i18n.Spanish {
		word = "sí"
	}
	if _, err := fmt.Fprintf(p.Out, "%s %s: ", p.Localize.Text(i18n.ConfirmPrompt, action), p.Localize.Text(i18n.ConfirmHint, word)); err != nil {
		return err
	}
	line, err := p.readLine()
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(line), word) {
		_, _ = fmt.Fprintln(p.Out, p.Localize.Text(i18n.Aborted))
		return ErrAborted
	}
	return nil
}

type Choice struct {
	Label string
	Value string
}

// Select resolves ambiguity only for a human terminal and returns the stable,
// untranslated value rather than the displayed label.
func (p *Prompter) Select(choices []Choice) (string, error) {
	if len(choices) == 0 {
		return "", ErrAborted
	}
	if len(choices) == 1 {
		return choices[0].Value, nil
	}
	if !p.Terminal {
		return "", ErrNonInteractive
	}
	if _, err := fmt.Fprintln(p.Out, p.Localize.Text(i18n.AmbiguousPrompt)); err != nil {
		return "", err
	}
	for index, choice := range choices {
		if _, err := fmt.Fprintf(p.Out, "  %d. %s\n", index+1, choice.Label); err != nil {
			return "", err
		}
	}
	if _, err := fmt.Fprintf(p.Out, "%s: ", p.Localize.Text(i18n.AmbiguousHint, len(choices))); err != nil {
		return "", err
	}
	line, err := p.readLine()
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	index, conversionErr := strconv.Atoi(strings.TrimSpace(line))
	if conversionErr != nil || index < 1 || index > len(choices) {
		_, _ = fmt.Fprintln(p.Out, p.Localize.Text(i18n.InvalidChoice))
		return "", ErrAborted
	}
	return choices[index-1].Value, nil
}

func (p *Prompter) readLine() (string, error) {
	if p.reader == nil {
		p.reader = bufio.NewReader(p.In)
	}
	line, err := p.reader.ReadString('\n')
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), err
}
