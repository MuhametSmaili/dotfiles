package flow

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"fzp/internal/proc"
)

const (
	builtinTimeout  = 10 * time.Second
	maxSourceOutput = 4 << 20
)

// source lists the choices of one step. It may read earlier selections and
// the target from env.
type source interface {
	list(ctx context.Context, env *env) ([]item, error)
}

// builtins maps a builtin name to a constructor that decodes and validates
// its with: parameters when the flow loads.
var builtins = map[string]func(*yaml.Node) (source, error){
	"tmux.sessions": newTmuxSessions,
	"tmux.windows":  newTmuxWindows,
	"tmux.panes":    newTmuxPanes,
	"git.commits":   newGitCommits,
	"git.branches":  newGitBranches,
	"git.worktrees": newGitWorktrees,
}

func builtinNames() []string {
	names := make([]string, 0, len(builtins))
	for name := range builtins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// expandParam expands variables in a builtin string parameter.
func expandParam(env *env, name, value string) (string, error) {
	expanded, err := env.expand(value)
	if err != nil {
		return "", fmt.Errorf("with.%s: %w", name, err)
	}
	return expanded, nil
}

// commandSource runs a shell command and reads one item per output line.
//
// Without columns, each line is both the ID and the label. With columns, the
// line is split on tabs and the last column takes any remaining text. The
// first column is the ID; the label defaults to the other columns.
type commandSource struct {
	command     string
	columns     []string
	label       []string
	description string
	timeout     time.Duration
}

func (s commandSource) list(ctx context.Context, env *env) ([]item, error) {
	out, err := proc.Command{
		Argv:        proc.Shell(s.command),
		Dir:         env.target.Dir(),
		Env:         env.environ(),
		Timeout:     s.timeout,
		StdoutLimit: maxSourceOutput,
	}.Output(ctx)
	if err != nil {
		return nil, err
	}
	return s.parse(string(out))
}

func (s commandSource) parse(out string) ([]item, error) {
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	items := make([]item, 0, len(lines))

	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}

		parsed, err := s.parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("output line %d: %w", i+1, err)
		}
		items = append(items, parsed)
	}
	return items, nil
}

func (s commandSource) parseLine(line string) (item, error) {
	if len(s.columns) == 0 {
		return item{ID: line, Label: line, Fields: map[string]string{"line": line}}, nil
	}

	values := strings.SplitN(line, "\t", len(s.columns))
	if len(values) != len(s.columns) {
		return item{}, fmt.Errorf("expected %d tab-separated columns, found %d", len(s.columns), len(values))
	}

	fields := make(map[string]string, len(values))
	for i, column := range s.columns {
		fields[column] = values[i]
	}

	label := s.label
	if len(label) == 0 {
		label = s.columns[min(1, len(s.columns)-1):]
	}

	parsed := item{
		ID:          values[0],
		Label:       joinColumns(fields, label),
		Description: fields[s.description],
		Fields:      fields,
	}
	return parsed, nil
}

func joinColumns(fields map[string]string, columns []string) string {
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		if value := fields[column]; value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "  ")
}
