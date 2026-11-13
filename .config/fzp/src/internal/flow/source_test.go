package flow

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"fzp/internal/tmux"
)

func TestCommandSourceParse(t *testing.T) {
	tests := []struct {
		name   string
		source commandSource
		output string
		want   []item
	}{
		{
			name:   "plain lines are id and label",
			source: commandSource{},
			output: "alpha\n\nbeta\r\n",
			want: []item{
				{ID: "alpha", Label: "alpha", Fields: map[string]string{"line": "alpha"}},
				{ID: "beta", Label: "beta", Fields: map[string]string{"line": "beta"}},
			},
		},
		{
			name:   "label defaults to the columns after the id",
			source: commandSource{columns: []string{"hash", "date", "subject"}},
			output: "abc\t2026-11-12\tfix: tabs\tstay in the last column\n",
			want: []item{{
				ID:    "abc",
				Label: "2026-11-12  fix: tabs\tstay in the last column",
				Fields: map[string]string{
					"hash": "abc", "date": "2026-11-12", "subject": "fix: tabs\tstay in the last column",
				},
			}},
		},
		{
			name:   "explicit label and description",
			source: commandSource{columns: []string{"id", "name", "path"}, label: []string{"name"}, description: "path"},
			output: "1\tapi\t/work/api\n",
			want: []item{{
				ID:          "1",
				Label:       "api",
				Description: "/work/api",
				Fields:      map[string]string{"id": "1", "name": "api", "path": "/work/api"},
			}},
		},
		{
			name:   "single column is its own label",
			source: commandSource{columns: []string{"path"}},
			output: "/work\n",
			want:   []item{{ID: "/work", Label: "/work", Fields: map[string]string{"path": "/work"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.source.parse(tt.output)

			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parse =\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

func TestCommandSourceParseRejectsMissingColumns(t *testing.T) {
	source := commandSource{columns: []string{"id", "name"}}

	_, err := source.parse("ok\tfine\nbroken\n")

	if err == nil || !strings.Contains(err.Error(), "output line 2") {
		t.Fatalf("error = %v, want a line 2 error", err)
	}
}

func TestCommandSourceSeesSelectionsAndRunsInTargetDirectory(t *testing.T) {
	dir := t.TempDir()
	env := newEnv(os.Environ(), tmux.Target{PaneID: "%1", Cwd: dir})
	env.push("project", item{ID: "api", Label: "API", Fields: map[string]string{"path": "/work/api"}})
	source := commandSource{
		command: `printf '%s\t%s\t%s\n' "$PWD" "$FZP_PANE_ID" "$FZP_STEP_PROJECT_PATH"`,
		columns: []string{"cwd", "pane", "project"},
		timeout: defaultSourceTimeout,
	}

	items, err := source.list(context.Background(), env)

	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := items[0].Fields
	if canonicalPath(got["cwd"]) != canonicalPath(dir) || got["pane"] != "%1" || got["project"] != "/work/api" {
		t.Errorf("fields = %v", got)
	}
}
