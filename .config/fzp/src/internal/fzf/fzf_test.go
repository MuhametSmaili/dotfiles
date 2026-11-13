package fzf

import "testing"

func TestParseOutput(t *testing.T) {
	tests := []struct {
		name   string
		out    string
		expect bool
		want   Result
	}{
		{name: "query and line", out: "ap\napi\n", want: Result{Query: "ap", Line: "api"}},
		{name: "no match", out: "zzz\n", want: Result{Query: "zzz"}},
		{name: "expected key", out: "q\nesc\napi\n", expect: true, want: Result{Query: "q", Key: "esc", Line: "api"}},
		{name: "enter with expect", out: "\n\napi\n", expect: true, want: Result{Line: "api"}},
		{name: "line keeps tabs", out: "\n0\tid\tlabel\n", want: Result{Line: "0\tid\tlabel"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOutput(tt.out, tt.expect)

			if got != tt.want {
				t.Errorf("parseOutput = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	got := Sanitize("a\tb\x1b[31m\nc\xff", true)

	if got != "a b [31m\nc\uFFFD" {
		t.Errorf("Sanitize = %q", got)
	}
}
