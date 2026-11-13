package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestListFindsFilesAndBundles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ssh.yaml"), "metadata: {description: SSH hosts}\nsource: {command: ls}")
	writeFile(t, filepath.Join(dir, "split.yaml"), "metadata: {group: pane}\nrun: {command: true}")
	writeFile(t, filepath.Join(dir, "files", "main.yaml"), "source: {script: ./source.sh}")
	writeFile(t, filepath.Join(dir, "files.yaml"), "source: {command: ignored}")
	writeFile(t, filepath.Join(dir, "notes.txt"), "not a channel")

	channels, err := List(dir)

	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var got []string
	for _, c := range channels {
		got = append(got, c.Name+":"+filepath.Base(c.Path)+":"+c.Metadata.Group)
	}
	if strings.Join(got, " ") != "files:main.yaml: split:split.yaml:pane ssh:ssh.yaml:" {
		t.Errorf("channels = %v", got)
	}

	if channels[2].Metadata.Description != "SSH hosts" {
		t.Errorf("metadata = %+v", channels[2].Metadata)
	}
}

func TestListSkipsUnreadableChannels(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "good.yaml"), "run: true")
	writeFile(t, filepath.Join(dir, "broken.yaml"), "metadata: [not, a, map")

	channels, err := List(dir)

	if len(channels) != 1 || channels[0].Name != "good" {
		t.Errorf("channels = %+v", channels)
	}
	if err == nil || !strings.Contains(err.Error(), "broken.yaml") {
		t.Errorf("error = %v, want one naming broken.yaml", err)
	}
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "files", "main.yaml"), "source: {command: ls}")

	tests := []struct {
		name    string
		channel string
		want    string
	}{
		{name: "bundle", channel: "files", want: ""},
		{name: "missing", channel: "nope", want: "channel not found"},
		{name: "path traversal", channel: "../files", want: "invalid channel name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Find(dir, tt.channel)

			if tt.want == "" && err != nil {
				t.Errorf("Find: %v", err)
			}
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Errorf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		meta    Metadata
		environ []string
		want    string
	}{
		{name: "satisfied", meta: Metadata{Requires: []string{"sh"}, RequiresEnv: []string{"FZP_PANE_ID"}}, environ: []string{"FZP_PANE_ID=%1"}},
		{name: "missing tool", meta: Metadata{Requires: []string{"sh", "fzp-surely-missing-tool"}}, want: "missing dependencies: fzp-surely-missing-tool"},
		{name: "unset variable", meta: Metadata{RequiresEnv: []string{"FZP_PANE_ID", "HOME"}}, environ: []string{"HOME=/h"}, want: "missing environment: FZP_PANE_ID"},
		{name: "empty variable", meta: Metadata{RequiresEnv: []string{"FZP_PANE_ID"}}, environ: []string{"FZP_PANE_ID=%1", "FZP_PANE_ID="}, want: "missing environment: FZP_PANE_ID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.meta.Check(tt.environ)

			if tt.want == "" && err != nil {
				t.Errorf("Check = %v", err)
			}
			if tt.want != "" && (err == nil || err.Error() != tt.want) {
				t.Errorf("Check = %v, want %q", err, tt.want)
			}
		})
	}
}
