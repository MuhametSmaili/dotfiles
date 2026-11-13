package flow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fzp/internal/tmux"
)

// tmuxServer starts an isolated tmux server with sessions "work" (windows
// "editor" and "shell", the editor split in two) and "notes" (window "todo"). The socket
// lives under /tmp because macOS temp paths exceed the socket path limit.
func tmuxServer(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}

	dir, err := os.MkdirTemp("/tmp", "fzp-test")
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "s")

	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("tmux", append([]string{"-S", socket, "-f", os.DevNull}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("tmux %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socket, "kill-server").Run()
		_ = os.RemoveAll(dir)
	})

	run("new-session", "-d", "-s", "work", "-n", "editor", "-c", dir)
	run("split-window", "-d", "-t", "work:editor", "-c", dir)
	run("new-window", "-d", "-t", "work:", "-n", "shell", "-c", dir)
	run("new-session", "-d", "-s", "notes", "-n", "todo", "-c", dir)
	return socket
}

func tmuxID(t *testing.T, socket, target, format string) string {
	t.Helper()

	out, err := exec.Command("tmux", "-S", socket, "display-message", "-p", "-t", target, format).Output()
	if err != nil {
		t.Fatalf("tmux display-message %s: %v", target, err)
	}
	return strings.TrimSpace(string(out))
}

func labels(t *testing.T, source source, env *env) []string {
	t.Helper()

	items, err := source.list(context.Background(), env)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Label)
	}
	return out
}

func TestTmuxBuiltins(t *testing.T) {
	socket := tmuxServer(t)
	work := tmuxID(t, socket, "work:", "#{session_id}")
	editor := tmuxID(t, socket, "work:editor", "#{window_id}")
	firstPane := tmuxID(t, socket, "work:editor.0", "#{pane_id}")
	target := tmux.Target{Socket: socket, SessionID: work, WindowID: editor, PaneID: firstPane}

	tests := []struct {
		name   string
		source source
		want   []string
	}{
		{name: "sessions", source: tmuxSessions{}, want: []string{"notes", "work"}},
		{name: "other sessions", source: tmuxSessions{ExcludeCurrent: true}, want: []string{"notes"}},
		{name: "all windows", source: tmuxWindows{}, want: []string{"notes / 0: todo", "work / 0: editor", "work / 1: shell"}},
		{name: "other windows in a session", source: tmuxWindows{Session: "$FZP_SESSION_ID", ExcludeCurrent: true}, want: []string{"1: shell"}},
		{name: "panes in a window", source: tmuxPanes{Window: "$FZP_WINDOW_ID", ExcludeCurrent: true}, want: []string{"0:editor.1 zsh"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newEnv(os.Environ(), target)

			got := labels(t, tt.source, env)

			if !slices.Equal(normalizeShell(got), tt.want) {
				t.Errorf("labels = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTmuxWindowsExposeIDsForLaterSteps(t *testing.T) {
	socket := tmuxServer(t)
	env := newEnv(os.Environ(), tmux.Target{Socket: socket})

	items, err := tmuxWindows{Session: "notes"}.list(context.Background(), env)

	if err != nil || len(items) != 1 {
		t.Fatalf("items = %v, err = %v", items, err)
	}
	fields := items[0].Fields
	if !strings.HasPrefix(fields["session_id"], "$") || !strings.HasPrefix(fields["pane_id"], "%") || fields["session_name"] != "notes" {
		t.Errorf("fields = %v", fields)
	}
}

func TestExcludeCurrentNeedsATarget(t *testing.T) {
	env := newEnv(os.Environ(), tmux.Target{})

	_, err := tmuxSessions{ExcludeCurrent: true}.list(context.Background(), env)

	if err != errNoTarget {
		t.Errorf("error = %v, want errNoTarget", err)
	}
}

// normalizeShell replaces the name of the user's shell in window and pane
// labels with "zsh", so the expectations do not depend on $SHELL.
func normalizeShell(labels []string) []string {
	shell := filepath.Base(os.Getenv("SHELL"))
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		if shell != "" {
			label = strings.ReplaceAll(label, shell, "zsh")
		}
		out = append(out, label)
	}
	return out
}
