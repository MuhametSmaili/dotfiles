package screen

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fzp/internal/catalog"
)

func loadSpec(t *testing.T, body string) (Spec, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "main.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(catalog.Channel{Name: "test", Path: path})
}

// flagValues returns every value given to flag, in order.
func flagValues(args []string, flag string) []string {
	var values []string
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			values = append(values, args[i+1])
		}
	}
	return values
}

func TestArgs(t *testing.T) {
	spec, err := loadSpec(t, `
source:
  script: ./source.sh
  delimiter: "\t"
  ansi: true
preview:
  command: "bat {2}"
ui:
  header: files
navigation:
  ctrl-r: {channel: git-log, label: git log}
actions:
  enter: {mode: become, label: edit, command: "nvim {2}"}
  f3: {mode: become, script: ./paste.sh, args: "{2}"}
  ctrl-g: {mode: raw, label: grep, command: select-all+accept}
fzf_options: [--tabstop=1]
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	args := spec.Args(Options{Channel: "files", Dir: "/ch", Query: "q", Height: "40%", Self: "/bin/fzp"})

	binds := flagValues(args, "--bind")
	want := []string{
		"ctrl-s:become:'/bin/fzp'",
		"ctrl-r:become:'/bin/fzp' 'git-log'",
		"ctrl-g:select-all+accept",
		"enter:become(nvim {2})",
		"f3:become('/ch/paste.sh' {2})",
	}
	if !slices.Equal(binds, want) {
		t.Errorf("binds =\n%q\nwant\n%q", binds, want)
	}

	header := flagValues(args, "--header")
	if len(header) != 1 || header[0] != "files | ctrl-s:channels | ctrl-r:git log | ctrl-g:grep | enter:edit | f3:f3" {
		t.Errorf("header = %q", header)
	}

	for _, flag := range []string{"--multi", "--ansi", "--tabstop=1"} {
		if !slices.Contains(args, flag) {
			t.Errorf("args are missing %s", flag)
		}
	}

	if got := flagValues(args, "--prompt"); got[0] != "[files]> " {
		t.Errorf("prompt = %q", got)
	}
}

func TestSourceCommandResolvesScripts(t *testing.T) {
	tests := []struct {
		name   string
		source Source
		want   string
	}{
		{name: "command", source: Source{Script: Script{Command: "ls -1"}}, want: "ls -1"},
		{name: "relative script", source: Source{Script: Script{Script: "./it's.sh", Args: "-a"}}, want: `'/ch/it'\''s.sh' -a`},
		{name: "absolute script", source: Source{Script: Script{Script: "/bin/list"}}, want: "'/bin/list'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.source.shell("/ch", "")

			if got != tt.want {
				t.Errorf("shell = %q, want %q", got, tt.want)
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
		{name: "unknown key", body: "source: {command: ls}\nui: {heigth: 10}", want: "field heigth not found"},
		{name: "missing source", body: "ui: {header: x}", want: "source: command or script is required"},
		{name: "command and script", body: "source: {command: ls, script: ./x}", want: "not both"},
		{name: "invalid mode", body: "source: {command: ls}\nactions: {enter: {mode: run, command: x}}", want: `action enter: invalid mode "run"`},
		{name: "action without command", body: "source: {command: ls}\nactions: {enter: {mode: become}}", want: "command or script is required"},
		{name: "navigation without channel", body: "source: {command: ls}\nnavigation: {ctrl-o: {label: x}}", want: "channel is required"},
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
