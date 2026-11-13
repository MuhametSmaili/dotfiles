package flow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fzp/internal/catalog"
)

func writeFlow(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "flow.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFlowCompilesStepsItemsAndRun(t *testing.T) {
	path := writeFlow(t, `
metadata:
  description: Split pane
  group: pane
  target: required
steps:
  - select: layout
    title: Choose split direction
    items:
      - {id: horizontal, label: Side by side, flag: -h, focus: true}
      - {id: vertical, flag: -v}
  - select: window
    source:
      builtin: tmux.windows
      with: {session: $FZP_SESSION_ID, exclude_current: true}
run:
  command: tmux split-window "$FZP_STEP_LAYOUT_FLAG"
  timeout: 5s
`)

	def, err := Load(catalog.Channel{Path: path})

	if err != nil {
		t.Fatalf("loadFlow: %v", err)
	}

	layout := def.steps[0]
	if layout.title != "Choose split direction" || len(layout.items) != 2 {
		t.Fatalf("layout step = %+v", layout)
	}

	first := layout.items[0]
	if first.Label != "Side by side" || first.Fields["flag"] != "-h" || first.Fields["focus"] != "true" {
		t.Errorf("first item = %+v", first)
	}

	if layout.items[1].Label != "vertical" {
		t.Errorf("label should default to id, got %q", layout.items[1].Label)
	}

	windows, ok := def.steps[1].source.(tmuxWindows)
	if !ok || windows.Session != "$FZP_SESSION_ID" || !windows.ExcludeCurrent {
		t.Errorf("window source = %#v", def.steps[1].source)
	}

	if def.steps[1].title != "window" {
		t.Errorf("title should default to the step name, got %q", def.steps[1].title)
	}

	if def.run.timeout != 5*time.Second || def.Metadata.Target != catalog.TargetRequired {
		t.Errorf("run = %+v, metadata = %+v", def.run, def.Metadata)
	}
}

func TestLoadFlowRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing run command",
			body: "steps: [{select: a, items: [{id: x}]}]",
			want: "run.command is required",
		},
		{
			name: "unknown top-level key",
			body: "run: {command: true}\nstep: []",
			want: "field step not found",
		},
		{
			name: "invalid step name",
			body: "steps: [{select: Bad-Name, items: [{id: x}]}]\nrun: {command: true}",
			want: "select must match",
		},
		{
			name: "duplicate step name",
			body: "steps: [{select: a, items: [{id: x}]}, {select: a, items: [{id: y}]}]\nrun: {command: true}",
			want: `duplicate step name "a"`,
		},
		{
			name: "step without choices",
			body: "steps: [{select: a}]\nrun: {command: true}",
			want: "needs items, a source, or both",
		},
		{
			name: "duplicate item id",
			body: "steps: [{select: a, items: [{id: x}, {id: x}]}]\nrun: {command: true}",
			want: `duplicate id "x"`,
		},
		{
			name: "nested item value",
			body: "steps: [{select: a, items: [{id: x, extra: {deep: 1}}]}]\nrun: {command: true}",
			want: "must be a scalar",
		},
		{
			name: "unknown builtin",
			body: "steps: [{select: a, source: {builtin: tmux.clients}}]\nrun: {command: true}",
			want: `unknown builtin "tmux.clients"`,
		},
		{
			name: "unknown builtin parameter",
			body: "steps: [{select: a, source: {builtin: tmux.sessions, with: {session: x}}}]\nrun: {command: true}",
			want: "field session not found",
		},
		{
			name: "builtin and command together",
			body: "steps: [{select: a, source: {builtin: tmux.sessions, command: ls}}]\nrun: {command: true}",
			want: "not both",
		},
		{
			name: "label refers to unknown column",
			body: "steps: [{select: a, source: {command: ls, columns: [id, name], label: [title]}}]\nrun: {command: true}",
			want: `unknown column "title"`,
		},
		{
			name: "interactive command with timeout",
			body: "run: {command: nvim, interactive: true, timeout: 1s}",
			want: "does not apply to interactive",
		},
		{
			name: "invalid timeout",
			body: "run: {command: true, timeout: soon}",
			want: `invalid duration "soon"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFlow(t, tt.body)

			_, err := Load(catalog.Channel{Path: path})

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}
