// Package output renders human-facing command results. JSON output is owned by
// internal/cli and deliberately bypasses this package.
package output

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/DaikuFi/daiku-cli/internal/i18n"
)

const (
	cyan  = "\x1b[36m"
	bold  = "\x1b[1m"
	reset = "\x1b[0m"
)

type Cell struct {
	Label string
	Value string
}

type Row []Cell

type Renderer struct {
	Writer   io.Writer
	Localize i18n.Localizer
	Terminal bool
	Width    int
	NoColor  bool
}

func (r Renderer) Empty() error {
	_, err := fmt.Fprintln(r.Writer, r.Localize.Text(i18n.NoResults))
	return err
}

// Table uses aligned columns when they fit and a label/value layout in narrow
// terminals. Cells are never truncated, keeping identifiers copyable.
func (r Renderer) Table(rows []Row) error {
	if len(rows) == 0 {
		return r.Empty()
	}
	labels := tableLabels(rows)
	if r.Width > 0 && tableWidth(rows, labels) > r.Width {
		return r.vertical(rows)
	}
	widths := columnWidths(rows, labels)
	for index, label := range labels {
		if index > 0 {
			if _, err := fmt.Fprint(r.Writer, "  "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(r.Writer, r.style(pad(label, widths[index]), bold)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(r.Writer); err != nil {
		return err
	}
	for _, row := range rows {
		values := valuesFor(row, labels)
		for index, value := range values {
			if index > 0 {
				if _, err := fmt.Fprint(r.Writer, "  "); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprint(r.Writer, pad(value, widths[index])); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(r.Writer); err != nil {
			return err
		}
	}
	return nil
}

func (r Renderer) vertical(rows []Row) error {
	for rowIndex, row := range rows {
		if rowIndex > 0 {
			if _, err := fmt.Fprintln(r.Writer); err != nil {
				return err
			}
		}
		for _, cell := range row {
			if _, err := fmt.Fprintf(r.Writer, "%s: %s\n", r.style(cell.Label, bold+cyan), cell.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r Renderer) style(value, code string) string {
	if !r.Terminal || r.NoColor {
		return value
	}
	return code + value + reset
}

func tableLabels(rows []Row) []string {
	seen := map[string]bool{}
	var labels []string
	for _, row := range rows {
		for _, cell := range row {
			if !seen[cell.Label] {
				seen[cell.Label] = true
				labels = append(labels, cell.Label)
			}
		}
	}
	return labels
}

func valuesFor(row Row, labels []string) []string {
	byLabel := make(map[string]string, len(row))
	for _, cell := range row {
		byLabel[cell.Label] = cell.Value
	}
	values := make([]string, len(labels))
	for index, label := range labels {
		values[index] = byLabel[label]
	}
	return values
}

func columnWidths(rows []Row, labels []string) []int {
	widths := make([]int, len(labels))
	for index, label := range labels {
		widths[index] = utf8.RuneCountInString(label)
	}
	for _, row := range rows {
		for index, value := range valuesFor(row, labels) {
			if width := utf8.RuneCountInString(value); width > widths[index] {
				widths[index] = width
			}
		}
	}
	return widths
}

func tableWidth(rows []Row, labels []string) int {
	widths := columnWidths(rows, labels)
	total := 2 * (len(widths) - 1)
	for _, width := range widths {
		total += width
	}
	return total
}

func pad(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-utf8.RuneCountInString(value)))
}
