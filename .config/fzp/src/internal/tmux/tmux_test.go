package tmux

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFields(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{name: "plain", line: "a|b|c", want: []string{"a", "b", "c"}},
		{name: "escaped separator", line: `a\|b|c`, want: []string{"a|b", "c"}},
		{name: "escaped backslash", line: `a\\|b`, want: []string{`a\`, "b"}},
		{name: "empty fields", line: "||", want: []string{"", "", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Fields(tt.line, len(tt.want))

			if err != nil || !slices.Equal(got, tt.want) {
				t.Errorf("Fields(%q) = %q, %v; want %q", tt.line, got, err, tt.want)
			}
		})
	}
}

func TestFieldsRejectsMalformedRows(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{name: "wrong field count", line: "a|b"},
		{name: "dangling escape", line: `a|b|c\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Fields(tt.line, 3)

			if err == nil {
				t.Errorf("Fields(%q) succeeded", tt.line)
			}
		})
	}
}

func TestResolveDescribesThePane(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}

	// macOS temp paths exceed the socket path limit.
	dir, err := os.MkdirTemp("/tmp", "fzp-test")
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "s")
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socket, "kill-server").Run()
		_ = os.RemoveAll(dir)
	})

	start := exec.Command("tmux", "-S", socket, "-f", os.DevNull, "new-session", "-d", "-s", "a|b", "-c", dir)
	if out, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start tmux: %v\n%s", err, out)
	}
	t.Setenv("TMUX", socket+",1,0")

	got, err := Resolve(context.Background(), "a|b:", "/dev/ttys001")

	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.HasPrefix(got.PaneID, "%") || !strings.HasPrefix(got.WindowID, "@") || !strings.HasPrefix(got.SessionID, "$") {
		t.Errorf("IDs = %+v", got)
	}
	if got.SessionName != "a|b" || got.Client != "/dev/ttys001" || got.Dir() == "" {
		t.Errorf("target = %+v", got)
	}
}

func TestResolveWithoutAPaneKeepsTheClient(t *testing.T) {
	got, err := Resolve(context.Background(), "", "/dev/ttys001")

	if err != nil || got != (Target{Client: "/dev/ttys001"}) {
		t.Errorf("Resolve = %+v, %v", got, err)
	}
}
