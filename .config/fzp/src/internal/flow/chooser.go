package flow

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"fzp/internal/fzf"
)

// fzfChooser shows screens with fzf. Each input line is
// "<index>\t<id>\t<display>"; only the display is shown, and the index maps
// the selection back to the item.
type fzfChooser struct {
	height string
	prompt string
}

const maxLabelWidth = 60

func (f fzfChooser) choose(ctx context.Context, s screen) (answer, error) {
	result, err := fzf.Pick(ctx, fzf.Prompt{
		Args:   f.args(s),
		Input:  renderLines(s.items),
		Expect: []string{"ctrl-r", "esc", "ctrl-c"},
		Env:    s.env,
		Dir:    s.dir,
	})
	if err != nil {
		return answer{}, err
	}
	return toAnswer(result, len(s.items))
}

func (f fzfChooser) args(s screen) []string {
	args := []string{
		"--height", f.height,
		"--prompt", f.prompt,
		"--header", fzf.Sanitize(s.header, true),
		"--query", s.query,
		"--with-shell", "bash -c",
		"--delimiter", "\t",
		"--with-nth", "3..",
		"--ansi",
		"--no-multi",
		"--bind", "enter:accept-non-empty",
	}

	if s.preview.Command != "" {
		args = append(args,
			"--preview", strings.ReplaceAll(s.preview.Command, "{id}", "{2}"),
			"--preview-window", firstNonEmpty(s.preview.Window, "right:50%"))
	}
	return args
}

func toAnswer(result fzf.Result, count int) (answer, error) {
	if result.Aborted {
		return answer{outcome: quit}, nil
	}

	switch result.Key {
	case "esc":
		return answer{outcome: back, query: result.Query}, nil
	case "ctrl-c":
		return answer{outcome: quit, query: result.Query}, nil
	case "ctrl-r":
		return answer{outcome: reload, query: result.Query}, nil
	}

	if result.Line == "" {
		return answer{outcome: reload, query: result.Query}, nil
	}

	index, _, _ := strings.Cut(result.Line, "\t")
	n, err := strconv.Atoi(index)
	if err != nil || n < 0 || n >= count {
		return answer{}, fmt.Errorf("fzf returned an unknown line %q", result.Line)
	}
	return answer{outcome: picked, index: n, query: result.Query}, nil
}

func renderLines(items []item) string {
	width := 0
	for _, it := range items {
		width = max(width, min(utf8.RuneCountInString(fzf.Sanitize(it.Label, false)), maxLabelWidth))
	}

	var b strings.Builder
	for i, it := range items {
		label := fzf.Sanitize(it.Label, false)
		b.WriteString(strconv.Itoa(i))
		b.WriteByte('\t')
		b.WriteString(fzf.Sanitize(it.ID, false))
		b.WriteByte('\t')
		b.WriteString(label)

		if it.Description != "" {
			padding := max(width-utf8.RuneCountInString(label), 0) + 2
			b.WriteString(strings.Repeat(" ", padding))
			b.WriteString("\x1b[2m")
			b.WriteString(fzf.Sanitize(it.Description, false))
			b.WriteString("\x1b[0m")
		}
		b.WriteByte('\n')
	}
	return b.String()
}
