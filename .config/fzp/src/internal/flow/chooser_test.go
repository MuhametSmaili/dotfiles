package flow

import (
	"strings"
	"testing"

	"fzp/internal/fzf"
)

func TestToAnswer(t *testing.T) {
	tests := []struct {
		name   string
		result fzf.Result
		want   answer
	}{
		{name: "enter on an item", result: fzf.Result{Query: "ap", Line: "1\tid\tapi"}, want: answer{outcome: picked, index: 1, query: "ap"}},
		{name: "esc", result: fzf.Result{Query: "q", Key: "esc", Line: "0\tid\tapi"}, want: answer{outcome: back, query: "q"}},
		{name: "ctrl-c", result: fzf.Result{Key: "ctrl-c"}, want: answer{outcome: quit}},
		{name: "aborted", result: fzf.Result{Aborted: true}, want: answer{outcome: quit}},
		{name: "ctrl-r", result: fzf.Result{Query: "x", Key: "ctrl-r"}, want: answer{outcome: reload, query: "x"}},
		{name: "no match", result: fzf.Result{Query: "zzz"}, want: answer{outcome: reload, query: "zzz"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toAnswer(tt.result, 2)

			if err != nil || got != tt.want {
				t.Errorf("toAnswer = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

func TestToAnswerRejectsUnknownIndex(t *testing.T) {
	_, err := toAnswer(fzf.Result{Line: "5\tid\tlabel"}, 2)

	if err == nil {
		t.Fatal("expected an error for an index outside the item list")
	}
}

func TestRenderLinesAlignsDescriptionsAndStripsControls(t *testing.T) {
	items := []item{
		{ID: "a", Label: "short", Description: "one"},
		{ID: "b\tc", Label: "longer\x1b[31m", Description: "two\nlines"},
		{ID: "d", Label: "none"},
	}

	got := strings.Split(strings.TrimSuffix(renderLines(items), "\n"), "\n")

	want := []string{
		"0\ta\tshort        \x1b[2mone\x1b[0m",
		"1\tb c\tlonger [31m  \x1b[2mtwo lines\x1b[0m",
		"2\td\tnone",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("renderLines =\n%q\nwant\n%q", got, want)
	}
}
