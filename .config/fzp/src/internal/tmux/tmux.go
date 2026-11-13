// Package tmux resolves the pane fzp acts on and runs tmux list queries.
package tmux

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"fzp/internal/proc"
)

const (
	queryTimeout = 10 * time.Second
	maxOutput    = 4 << 20
)

// Target is the tmux pane fzp acts on and the client that opened fzp. Empty
// fields are unknown.
type Target struct {
	PaneID      string
	WindowID    string
	SessionID   string
	SessionName string
	Cwd         string
	Socket      string
	Client      string
}

// Resolve describes pane, a pane ID or any tmux target string, on the server
// fzp runs under. An empty pane yields a Target with only Client set.
func Resolve(ctx context.Context, pane, client string) (Target, error) {
	if pane == "" {
		return Target{Client: client}, nil
	}

	format := "#{pane_id}|#{window_id}|#{session_id}|#{q:session_name}|#{q:pane_current_path}|#{q:socket_path}"
	rows, err := Rows(ctx, "", nil, 6, "display-message", "-p", "-t", pane, format)
	if err != nil {
		return Target{}, fmt.Errorf("resolve tmux target %q: %w", pane, err)
	}

	if len(rows) != 1 {
		return Target{}, fmt.Errorf("resolve tmux target %q: expected one pane, found %d", pane, len(rows))
	}

	row := rows[0]
	target := Target{
		PaneID:      row[0],
		WindowID:    row[1],
		SessionID:   row[2],
		SessionName: row[3],
		Cwd:         row[4],
		Socket:      row[5],
		Client:      client,
	}
	return target, nil
}

// Environ returns the target as the FZP_* variables channels read.
func (t Target) Environ() []string {
	return []string{
		"FZP_PANE_ID=" + t.PaneID,
		"FZP_WINDOW_ID=" + t.WindowID,
		"FZP_SESSION_ID=" + t.SessionID,
		"FZP_SESSION_NAME=" + t.SessionName,
		"FZP_PANE_CWD=" + t.Cwd,
		"FZP_SOCKET=" + t.Socket,
		"FZP_CLIENT=" + t.Client,
	}
}

// Dir returns the target pane's directory if it still exists, else "".
func (t Target) Dir() string {
	if t.Cwd == "" {
		return ""
	}

	if stat, err := os.Stat(t.Cwd); err != nil || !stat.IsDir() {
		return ""
	}
	return t.Cwd
}

// Rows runs a tmux command whose format separates #{q:}-escaped fields with
// '|' and splits each output line into want fields. An empty socket means the
// server tmux itself would pick ($TMUX or the default socket).
func Rows(ctx context.Context, socket string, env []string, want int, args ...string) ([][]string, error) {
	argv := []string{"tmux"}
	if socket != "" {
		argv = append(argv, "-S", socket)
	}

	out, err := proc.Command{
		Argv:        append(argv, args...),
		Env:         env,
		Timeout:     queryTimeout,
		StdoutLimit: maxOutput,
	}.Output(ctx)
	if err != nil {
		return nil, err
	}

	var rows [][]string
	for i, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}

		fields, err := Fields(line, want)
		if err != nil {
			return nil, fmt.Errorf("tmux %s row %d: %w", args[0], i+1, err)
		}
		rows = append(rows, fields)
	}
	return rows, nil
}

// Fields splits one #{q:...}-escaped row: a backslash always introduces one
// literal byte, so an unescaped '|' is unambiguous.
func Fields(line string, want int) ([]string, error) {
	fields := make([]string, 0, want)
	var value strings.Builder

	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			if i+1 == len(line) {
				return nil, fmt.Errorf("dangling escape in row %q", line)
			}
			i++
			value.WriteByte(line[i])
		case '|':
			fields = append(fields, value.String())
			value.Reset()
		default:
			value.WriteByte(line[i])
		}
	}
	fields = append(fields, value.String())

	if len(fields) != want {
		return nil, fmt.Errorf("expected %d fields, found %d", want, len(fields))
	}
	return fields, nil
}
