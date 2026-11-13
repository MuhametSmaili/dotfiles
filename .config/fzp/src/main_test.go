package main

import (
	"io"
	"testing"
)

func TestParseFlagsDistinguishesAnEmptyQueryFromNone(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantInput *string
		wantFilt  *string
	}{
		{name: "absent", args: []string{"grep"}},
		{name: "empty input query", args: []string{"--input-query", "", "grep"}, wantInput: ptr("")},
		{name: "input query", args: []string{"--input-query", "src/ !test", "grep"}, wantInput: ptr("src/ !test")},
		{name: "empty filter", args: []string{"--filter", "", "files"}, wantFilt: ptr("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseFlags(tt.args, io.Discard, paths{})

			if err != nil {
				t.Fatalf("parseFlags: %v", err)
			}
			if !samePtr(opts.inputQuery, tt.wantInput) || !samePtr(opts.filter, tt.wantFilt) {
				t.Errorf("inputQuery = %v, filter = %v", opts.inputQuery, opts.filter)
			}
		})
	}
}

func TestParseFlagsRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "two channels", args: []string{"files", "grep"}},
		{name: "filter without a channel", args: []string{"--filter", "x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseFlags(tt.args, io.Discard, paths{})

			if err == nil {
				t.Errorf("parseFlags(%q) succeeded", tt.args)
			}
		})
	}
}

func TestWithoutInternalDropsStateFromAParentFzp(t *testing.T) {
	got := withoutInternal([]string{"HOME=/h", "FZP_INPUT_QUERY=old", "FZP_SCOPE_FILE=/tmp/x", "FZP_RESULT=/tmp/r", "FZP_PANE_ID=%1"})

	if len(got) != 2 || got[0] != "HOME=/h" || got[1] != "FZP_PANE_ID=%1" {
		t.Errorf("withoutInput = %q", got)
	}
}

func ptr(s string) *string { return &s }

func samePtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
