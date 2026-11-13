package channel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadSpec(t *testing.T, body string) (Spec, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "main.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func mustLoad(t *testing.T, body string) Spec {
	t.Helper()

	spec, err := loadSpec(t, body)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return spec
}

func TestLoadAcceptsValidChannels(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "one screen", body: "steps: [{source: ls, keys: {enter: {do: 'become:vi {}'}}}]"},
		{name: "run only", body: "run: gh browse"},
		{name: "items and run", body: `
steps:
  - select: layout
    columns: [flag, label]
    show: [label]
    items: [{flag: -h, label: Side by side}]
confirm: Split with $FZP_STEP_LAYOUT_FLAG?
run: tmux split-window "$FZP_STEP_LAYOUT_FLAG"
`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadSpec(t, tt.body)

			if err != nil {
				t.Errorf("Load: %v", err)
			}
		})
	}
}

func TestLoadRejectsInvalidChannels(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "empty", body: "metadata: {description: x}", want: "steps or run is required"},
		{name: "unknown key", body: "steps: [{source: ls, sorce: x}]", want: "field sorce not found"},
		{name: "step without rows", body: "steps: [{title: x}]", want: "step 1: source or items is required"},
		{name: "invalid select", body: "steps: [{select: Bad-Name, source: ls}]", want: "must match"},
		{name: "duplicate select", body: "steps: [{select: a, source: ls}, {select: a, source: ls}]", want: `duplicate select "a"`},
		{name: "duplicate column", body: "steps: [{source: ls, columns: [a, a]}]", want: `duplicate column "a"`},
		{name: "show unknown column", body: "steps: [{source: ls, columns: [a], show: [b]}]", want: `show: unknown column "b"`},
		{name: "item unknown column", body: "steps: [{columns: [a], items: [{b: x}]}]", want: `item 1: unknown column "b"`},
		{name: "item with tab", body: "steps: [{columns: [a], items: [{a: \"x\\ty\"}]}]", want: "must not contain tabs"},
		{name: "nested item value", body: "steps: [{columns: [a], items: [{a: {b: c}}]}]", want: "cannot unmarshal"},
		{name: "key without action", body: "steps: [{source: ls, keys: {enter: {label: x}}}]", want: "key enter: do is required"},
		{name: "confirm without run", body: "steps: [{source: ls}]\nconfirm: sure?", want: "confirm needs run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadSpec(t, tt.body)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}
