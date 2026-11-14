package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"fzp/internal/catalog"
)

func TestMenuStepGroupsChannelsAndKeepsUngroupedChannels(t *testing.T) {
	channels := []catalog.Channel{
		{Name: "git-log", Path: "/git-log.yaml", Metadata: catalog.Metadata{Group: "Git", Description: "Commit\tlog\npreview"}},
		{Name: "git-branches", Metadata: catalog.Metadata{Group: "Git"}},
		{Name: "pane-split", Metadata: catalog.Metadata{Group: "Tmux"}},
		{Name: "custom", Metadata: catalog.Metadata{Description: "My channel"}},
	}

	root := menuStep(channels, "")
	want := []map[string]string{
		{"id": "custom", "name": "custom", "description": "My channel", "path": ""},
		{"id": "group:Git", "name": "Git", "description": "git-log, git-branches"},
		{"id": "group:Tmux", "name": "Tmux", "description": "pane-split"},
	}
	if !reflect.DeepEqual(root.Items, want) {
		t.Errorf("root items = %#v, want %#v", root.Items, want)
	}
	if root.Preview != "" {
		t.Error("the root menu should not preview category names as files")
	}

	group := menuStep(channels, "git")
	if len(group.Items) != 2 || group.Items[0]["id"] != "git-branches" || group.Items[1]["id"] != "git-log" {
		t.Fatalf("Git items = %#v", group.Items)
	}
	if group.Items[1]["description"] != "Commit log preview" {
		t.Errorf("description = %q", group.Items[1]["description"])
	}
	if !strings.Contains(group.Preview, "{4}") {
		t.Errorf("preview must use the hidden path column: %q", group.Preview)
	}
	if got := menuStep(channels, "missing"); len(got.Items) != 0 {
		t.Errorf("unknown group has items: %#v", got.Items)
	}
}

func TestConfiguredCategoriesContainEveryChannelOnce(t *testing.T) {
	channels, err := catalog.List("../channels.d")
	if err != nil {
		t.Fatal(err)
	}
	root := menuStep(channels, "")
	var names []string
	seen := map[string]bool{}
	for _, item := range root.Items {
		names = append(names, item["name"])
		for _, child := range menuStep(channels, item["name"]).Items {
			if seen[child["id"]] {
				t.Errorf("duplicate channel %q", child["id"])
			}
			seen[child["id"]] = true
		}
	}
	want := []string{"Docker", "Files", "Git", "Kubernetes", "Network", "System", "Tmux"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("categories = %q, want %q", names, want)
	}
	if len(seen) != len(channels) {
		t.Errorf("categories contain %d of %d channels", len(seen), len(channels))
	}
	git := menuStep(channels, "Git")
	if len(git.Items) != 8 {
		t.Errorf("Git should include lazygit and all Git actions, got %d", len(git.Items))
	}
}

func TestMenuNavigation(t *testing.T) {
	tests := []struct {
		name    string
		opts    options
		answers []string
		prompts []string
		queries []string
	}{
		{
			name:    "escape returns from channel to category to root",
			answers: []string{"pick\n\ngroup:Git", "pick\n\ngit-log", "back\n", "back\n", "back\n"},
			prompts: []string{"channels> ", "Git> ", "[git-log]> ", "Git> ", "channels> "},
		},
		{
			name:    "ctrl-c in a category exits all menus",
			answers: []string{"pick\n\ngroup:Git", ""},
			prompts: []string{"channels> ", "Git> "},
		},
		{
			name:    "ctrl-c in a channel exits all menus",
			answers: []string{"pick\n\ngroup:Git", "pick\n\ngit-log", ""},
			prompts: []string{"channels> ", "Git> ", "[git-log]> "},
		},
		{
			name:    "explicit group is the root",
			opts:    options{group: "git"},
			answers: []string{"back\n"},
			prompts: []string{"git> "},
		},
		{
			name:    "successful action closes the menu",
			answers: []string{"pick\n\ngroup:Git", "pick\n\ngit-done"},
			prompts: []string{"channels> ", "Git> "},
		},
		{
			name:    "initial category query does not filter the submenu",
			opts:    options{query: "Git"},
			answers: []string{"pick\n\ngroup:Git", "back\n", ""},
			prompts: []string{"channels> ", "Git> ", "channels> "},
			queries: []string{"Git", "", "Git"},
		},
		{
			name: "direct channel skips the menu",
			opts: options{args: []string{"git-done"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := fakeFzf(t, tt.answers)
			channels := filepath.Join(dir, "channels")
			if err := os.Mkdir(channels, 0700); err != nil {
				t.Fatal(err)
			}
			for name, spec := range map[string]string{
				"git-log":  "metadata: {group: Git}\nsteps: [{source: 'true'}]\n",
				"git-done": "metadata: {group: Git}\nrun: 'true'\n",
			} {
				writeTestFile(t, filepath.Join(channels, name+".yaml"), spec, 0600)
			}
			app := newApp(paths{config: dir, channels: channels}, tt.opts)
			if err := app.run(context.Background(), tt.opts); err != nil {
				t.Fatal(err)
			}
			if tt.prompts == nil {
				if _, err := os.Stat(filepath.Join(dir, "prompts")); !os.IsNotExist(err) {
					t.Fatalf("direct command unexpectedly opened a menu: %v", err)
				}
				return
			}
			data, err := os.ReadFile(filepath.Join(dir, "prompts"))
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"); !reflect.DeepEqual(got, tt.prompts) {
				t.Errorf("prompts = %q, want %q", got, tt.prompts)
			}
			if tt.queries != nil {
				data, err := os.ReadFile(filepath.Join(dir, "queries"))
				if err != nil {
					t.Fatal(err)
				}
				if got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"); !reflect.DeepEqual(got, tt.queries) {
					t.Errorf("queries = %q, want %q", got, tt.queries)
				}
			}
		})
	}
}

// fakeFzf exercises the real menu/channel flow without a terminal or actions
// against the user's sessions. Each invocation writes one result-file answer.
func fakeFzf(t *testing.T, answers []string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("FZP_TEST_DIR", dir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for i, answer := range answers {
		writeTestFile(t, filepath.Join(dir, strconv.Itoa(i)), answer, 0600)
	}
	writeTestFile(t, filepath.Join(dir, "fzf"), `#!/bin/bash
set -eu
dir=$FZP_TEST_DIR
count=0
[[ ! -f "$dir/count" ]] || read -r count < "$dir/count"
printf '%s\n' "$((count + 1))" > "$dir/count"
while [[ $# -gt 0 ]]; do
  if [[ $1 == --prompt ]]; then
    printf '%s\n' "$2" >> "$dir/prompts"
  fi
  if [[ $1 == --query ]]; then
    printf '%s\n' "$2" >> "$dir/queries"
  fi
  shift
done
cat "$dir/$count" > "$FZP_RESULT"
`, 0700)
	return dir
}

func writeTestFile(t *testing.T, path, data string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}
