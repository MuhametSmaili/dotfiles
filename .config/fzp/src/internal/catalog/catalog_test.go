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

	if channels[1].Kind != Flow || channels[2].Kind != Screen || channels[2].Metadata.Description != "SSH hosts" {
		t.Errorf("kinds or metadata = %+v", channels)
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

func TestReadRejectsUnknownTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x.yaml"), "metadata: {target: window}\nrun: {command: true}")

	_, err := Find(dir, "x")

	if err == nil || !strings.Contains(err.Error(), "metadata.target") {
		t.Errorf("error = %v", err)
	}
}

func TestMissingTools(t *testing.T) {
	m := Metadata{Requires: []string{"sh", "fzp-surely-missing-tool"}}

	err := m.MissingTools()

	if err == nil || err.Error() != "missing dependencies: fzp-surely-missing-tool" {
		t.Errorf("MissingTools = %v", err)
	}
}
