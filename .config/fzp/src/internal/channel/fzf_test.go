package channel

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestProducerListsItemsThenSource(t *testing.T) {
	step := Step{
		Columns: []string{"id", "label"},
		Items:   []map[string]string{{"id": "new", "label": "it's new"}, {"label": "no id"}},
		Source:  "printf 'a\\tb\\n'",
	}

	out, err := exec.Command("bash", "-c", step.producer()).Output()

	if err != nil {
		t.Fatalf("producer: %v", err)
	}
	if string(out) != "new\tit's new\n\tno id\na\tb\n" {
		t.Errorf("rows = %q", out)
	}
}

func TestArgs(t *testing.T) {
	s := screen{
		step: Step{
			Columns: []string{"id", "name", "path"},
			Show:    []string{"name", "path"},
			Preview: "cat {3}",
			Keys:    map[string]Key{"ctrl-y": {Do: "execute-silent:copy {1}", Label: "copy"}, "ctrl-r": {Do: "become:fzp other"}},
			Fzf:     []string{"--ansi"},
		},
		prompt: "[x]> ",
		height: "40%",
		query:  "q",
		help:   stepHelp,
	}

	args := s.args()

	want := [][]string{
		{"--with-nth", "2,3"},
		{"--preview", "cat {3}"},
		{"--bind", "ctrl-r:become:fzp other"},
		{"--bind", "ctrl-y:execute-silent:copy {1}"},
		{"--query", "q"},
	}
	for _, pair := range want {
		if !containsPair(args, pair[0], pair[1]) {
			t.Errorf("args are missing %q %q", pair[0], pair[1])
		}
	}

	if args[len(args)-1] != "--ansi" {
		t.Errorf("channel fzf options must come last, so they win: %q", args)
	}
}

func TestStartKeyRunsAfterTheRowsLoad(t *testing.T) {
	s := screen{step: Step{Keys: map[string]Key{"start": {Do: "change-prompt(x> )"}}}}

	args := s.args()

	if !containsPair(args, "--bind", `start:reload(eval "$FZP_SOURCE")+change-prompt(x> )`) {
		t.Errorf("start binding missing in %q", args)
	}
	if slices.Contains(args, "start:change-prompt(x> )") {
		t.Error("start must not be bound twice")
	}
}

func TestHeaderHidesBuiltinKeysTheStepRebinds(t *testing.T) {
	s := screen{
		step: Step{Title: "Pick", Keys: map[string]Key{"ctrl-r": {Do: "become:x", Label: "git log"}, "f2": {Do: "x"}}},
		help: stepHelp,
	}

	got := s.header()

	if got != "Pick\nenter select · esc back · ctrl-r git log" {
		t.Errorf("header = %q", got)
	}
}

func TestParseResult(t *testing.T) {
	s := screen{step: Step{Columns: []string{"id", "name"}}}

	tests := []struct {
		name string
		data string
		want answer
	}{
		{name: "pick", data: "pick\nqu ery\n1\tone\n2\ttwo\tand a tab\n", want: answer{outcome: picked, query: "qu ery", rows: [][]string{{"1", "one"}, {"2", "two\tand a tab"}}}},
		{name: "short row", data: "pick\n\n1\n", want: answer{outcome: picked, rows: [][]string{{"1", ""}}}},
		{name: "back", data: "back\nkept\n", want: answer{outcome: back, query: "kept"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.parse(tt.data)

			if got.outcome != tt.want.outcome || got.query != tt.want.query || !slices.EqualFunc(got.rows, tt.want.rows, slices.Equal) {
				t.Errorf("parse = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func containsPair(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func TestWithoutSteps(t *testing.T) {
	got := withoutSteps([]string{"HOME=/h", "FZP_STEP_A_ID=old"})

	if strings.Join(got, " ") != "HOME=/h" {
		t.Errorf("withoutSteps = %q", got)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{a: "0.75.0", b: "0.51.0", want: 1},
		{a: "0.51.0", b: "0.51.0", want: 0},
		{a: "0.44.1", b: "0.51.0", want: -1},
		{a: "0.9", b: "0.51.0", want: -1},
		{a: "1.0", b: "0.51.0", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.a+" vs "+tt.b, func(t *testing.T) {
			if got := compareVersions(tt.a, tt.b); got != tt.want {
				t.Errorf("compareVersions = %d, want %d", got, tt.want)
			}
		})
	}
}
