package channel

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func requireFzf(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("fzf"); err != nil {
		t.Skip("fzf is not installed")
	}
}

func TestFilterMatchesLikeTheStep(t *testing.T) {
	requireFzf(t)
	spec := mustLoad(t, `
steps:
  - columns: [icon, path]
    show: [path]
    items: [{icon: "*", path: README.md}]
    source: printf 'a\tmain.go\nb\tcmd/main_test.go\n'
`)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "fuzzy", query: "mngo", want: []string{"a\tmain.go", "b\tcmd/main_test.go"}},
		{name: "extended syntax", query: "main !test", want: []string{"a\tmain.go"}},
		{name: "only shown columns are searched", query: "a", want: []string{"*\tREADME.md", "a\tmain.go", "b\tcmd/main_test.go"}},
		{name: "empty query keeps every row", query: "", want: []string{"*\tREADME.md", "a\tmain.go", "b\tcmd/main_test.go"}},
		{name: "no match is not an error", query: "zzz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer

			err := spec.Filter(context.Background(), Options{Env: os.Environ()}, tt.query, &out)

			if err != nil {
				t.Fatalf("Filter: %v", err)
			}
			if got := sortedLines(out.String()); !slices.Equal(got, sortedLines(strings.Join(tt.want, "\n"))) {
				t.Errorf("rows = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterReportsAFailingSource(t *testing.T) {
	requireFzf(t)
	spec := mustLoad(t, "steps: [{source: 'echo broken >&2; exit 3'}]")

	err := spec.Filter(context.Background(), Options{Env: os.Environ()}, "", &bytes.Buffer{})

	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Errorf("error = %v, want the source's stderr", err)
	}
}

func TestPrepareScope(t *testing.T) {
	spec := mustLoad(t, "scope: printf '%s\\n' \"$FZP_INPUT_QUERY\" one\nsteps: [{source: 'true'}]")
	env := append(os.Environ(), "FZP_INPUT_QUERY=src/")

	t.Run("runs once with an input query and removes its file", func(t *testing.T) {
		got, err := spec.prepareScope(context.Background(), Options{Env: env, HasInput: true})

		if err != nil {
			t.Fatalf("prepareScope: %v", err)
		}
		data, err := os.ReadFile(got.path)
		if err != nil || string(data) != "src/\none\n" {
			t.Errorf("scope file = %q, %v", data, err)
		}

		got.remove()

		if _, err := os.Stat(got.path); !os.IsNotExist(err) {
			t.Errorf("scope file still exists: %v", err)
		}
	})

	t.Run("does nothing without an input query", func(t *testing.T) {
		got, err := spec.prepareScope(context.Background(), Options{Env: env})

		if err != nil || got.path != "" || got.env() != nil {
			t.Errorf("scope = %+v, %v; want none", got, err)
		}
	})
}

func TestPrepareScopeFailureLeavesNoFile(t *testing.T) {
	spec := mustLoad(t, "scope: echo no repo >&2; exit 1\nsteps: [{source: 'true'}]")
	t.Setenv("TMPDIR", t.TempDir())

	_, err := spec.prepareScope(context.Background(), Options{Env: os.Environ(), HasInput: true})

	if err == nil || !strings.Contains(err.Error(), "scope: command: exit status 1\nno repo") {
		t.Errorf("error = %v", err)
	}
	if left, _ := os.ReadDir(os.Getenv("TMPDIR")); len(left) != 0 {
		t.Errorf("left %d files behind", len(left))
	}
}

func sortedLines(out string) []string {
	if out == "" {
		return nil
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	slices.Sort(lines)
	return lines
}
