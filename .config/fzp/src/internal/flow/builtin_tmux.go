package flow

import (
	"context"
	"errors"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"fzp/internal/tmux"
)

var errNoTarget = errors.New("exclude_current needs a target pane (run fzp with --target)")

type tmuxSessions struct {
	ExcludeCurrent bool `yaml:"exclude_current"`
}

type tmuxWindows struct {
	Session        string `yaml:"session"`
	ExcludeCurrent bool   `yaml:"exclude_current"`
}

type tmuxPanes struct {
	Session        string `yaml:"session"`
	Window         string `yaml:"window"`
	ExcludeCurrent bool   `yaml:"exclude_current"`
}

func newTmuxSessions(with *yaml.Node) (source, error) {
	var s tmuxSessions
	return s, decodeParams(with, &s)
}

func newTmuxWindows(with *yaml.Node) (source, error) {
	var s tmuxWindows
	return s, decodeParams(with, &s)
}

func newTmuxPanes(with *yaml.Node) (source, error) {
	var s tmuxPanes
	if err := decodeParams(with, &s); err != nil {
		return nil, err
	}

	if s.Session != "" && s.Window != "" {
		return nil, errors.New("set session or window, not both")
	}
	return s, nil
}

func (s tmuxSessions) list(ctx context.Context, env *env) ([]item, error) {
	if s.ExcludeCurrent && env.target.SessionID == "" {
		return nil, errNoTarget
	}

	format := "#{session_id}|#{q:session_name}|#{session_windows}|#{session_attached}"
	rows, err := tmuxRows(ctx, env, 4, "list-sessions", "-F", format)
	if err != nil {
		return nil, err
	}

	items := make([]item, 0, len(rows))
	for _, row := range rows {
		id, name, windows, attached := row[0], row[1], row[2], row[3]
		if s.ExcludeCurrent && id == env.target.SessionID {
			continue
		}

		description := windows + " windows"
		if attached != "0" {
			description += ", attached"
		}

		items = append(items, item{
			ID:          id,
			Label:       name,
			Description: description,
			Fields:      map[string]string{"session_id": id, "session_name": name},
		})
	}
	return items, nil
}

func (s tmuxWindows) list(ctx context.Context, env *env) ([]item, error) {
	if s.ExcludeCurrent && env.target.WindowID == "" {
		return nil, errNoTarget
	}

	session, err := expandParam(env, "session", s.Session)
	if err != nil {
		return nil, err
	}

	scope := []string{"-a"}
	if session != "" {
		scope = []string{"-t", session}
	}

	format := "#{session_id}|#{q:session_name}|#{window_id}|#{window_index}|#{q:window_name}|#{pane_id}|#{q:pane_current_path}"
	args := append([]string{"list-windows"}, scope...)
	rows, err := tmuxRows(ctx, env, 7, append(args, "-F", format)...)
	if err != nil {
		return nil, err
	}

	items := make([]item, 0, len(rows))
	for _, row := range rows {
		fields := map[string]string{
			"session_id":   row[0],
			"session_name": row[1],
			"window_id":    row[2],
			"window_index": row[3],
			"window_name":  row[4],
			"pane_id":      row[5],
			"path":         row[6],
		}
		if s.ExcludeCurrent && fields["window_id"] == env.target.WindowID {
			continue
		}

		label := fields["window_index"] + ": " + fields["window_name"]
		if session == "" {
			label = fields["session_name"] + " / " + label
		}

		items = append(items, item{
			ID:          fields["window_id"],
			Label:       label,
			Description: shortenHome(fields["path"]),
			Fields:      fields,
		})
	}
	return items, nil
}

func (s tmuxPanes) list(ctx context.Context, env *env) ([]item, error) {
	if s.ExcludeCurrent && env.target.PaneID == "" {
		return nil, errNoTarget
	}

	scope, err := s.scope(env)
	if err != nil {
		return nil, err
	}

	format := "#{session_id}|#{q:session_name}|#{window_id}|#{window_index}|#{q:window_name}" +
		"|#{pane_id}|#{pane_index}|#{q:pane_current_command}|#{q:pane_current_path}"
	args := append([]string{"list-panes"}, scope...)
	rows, err := tmuxRows(ctx, env, 9, append(args, "-F", format)...)
	if err != nil {
		return nil, err
	}

	items := make([]item, 0, len(rows))
	for _, row := range rows {
		fields := map[string]string{
			"session_id":   row[0],
			"session_name": row[1],
			"window_id":    row[2],
			"window_index": row[3],
			"window_name":  row[4],
			"pane_id":      row[5],
			"pane_index":   row[6],
			"command":      row[7],
			"path":         row[8],
		}
		if s.ExcludeCurrent && fields["pane_id"] == env.target.PaneID {
			continue
		}

		label := fields["window_index"] + ":" + fields["window_name"] + "." + fields["pane_index"] + " " + fields["command"]
		if len(scope) == 1 {
			label = fields["session_name"] + " / " + label
		}

		items = append(items, item{
			ID:          fields["pane_id"],
			Label:       label,
			Description: shortenHome(fields["path"]),
			Fields:      fields,
		})
	}
	return items, nil
}

// scope returns the list-panes arguments that select the panes to list. A
// single "-a" means the whole server.
func (s tmuxPanes) scope(env *env) ([]string, error) {
	window, err := expandParam(env, "window", s.Window)
	if err != nil {
		return nil, err
	}

	session, err := expandParam(env, "session", s.Session)
	if err != nil {
		return nil, err
	}

	switch {
	case window != "":
		return []string{"-t", window}, nil
	case session != "":
		return []string{"-s", "-t", session}, nil
	default:
		return []string{"-a"}, nil
	}
}

// tmuxRows runs a tmux list query on the target's server.
func tmuxRows(ctx context.Context, env *env, want int, args ...string) ([][]string, error) {
	return tmux.Rows(ctx, env.target.Socket, env.environ(), want, args...)
}

func shortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}

	if path == home {
		return "~"
	}

	if rest, ok := strings.CutPrefix(path, home+"/"); ok {
		return "~/" + rest
	}
	return path
}
