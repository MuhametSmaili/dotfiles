// Package fzf runs fzf as a chooser and reads back what the user did.
package fzf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unicode"
)

// fzf's exit codes.
const (
	exitNoMatch = 1
	exitAborted = 130
)

// Prompt is one fzf invocation that reads its lines from Input.
type Prompt struct {
	Args  []string
	Input string
	// Expect lists keys that end fzf like Enter does; Result.Key reports
	// which one was pressed.
	Expect []string
	Env    []string
	Dir    string
}

// Result is what the user did. Line is empty when nothing matched.
type Result struct {
	Query   string
	Key     string
	Line    string
	Aborted bool
}

// Pick shows the prompt and waits for the user.
func Pick(ctx context.Context, p Prompt) (Result, error) {
	args := append(append([]string{}, p.Args...), "--print-query")
	if len(p.Expect) > 0 {
		args = append(args, "--expect="+strings.Join(p.Expect, ","))
	}

	cmd := exec.CommandContext(ctx, "fzf", args...)
	cmd.Dir = p.Dir
	cmd.Env = p.Env
	cmd.Stdin = strings.NewReader(p.Input)
	cmd.Stderr = os.Stderr
	// SIGTERM lets fzf restore the terminal; SIGKILL would leave it raw.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}

	code := exitCode(err)
	aborted := code == exitAborted
	if err != nil && code != exitAborted && code != exitNoMatch {
		return Result{}, fmt.Errorf("fzf: %w", err)
	}

	result := parseOutput(stdout.String(), len(p.Expect) > 0)
	result.Aborted = aborted
	return result, nil
}

// exitCode returns the exit code of a finished fzf, or -1 when err is not an
// exit status.
func exitCode(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}

// parseOutput reads the --print-query output: the query, the --expect key
// line when keys were given, and the selected line.
func parseOutput(out string, expect bool) Result {
	fields := 2
	if expect {
		fields = 3
	}

	lines := strings.SplitN(strings.TrimSuffix(out, "\n"), "\n", fields)
	for len(lines) < fields {
		lines = append(lines, "")
	}

	if !expect {
		return Result{Query: lines[0], Line: lines[1]}
	}
	return Result{Query: lines[0], Key: lines[1], Line: lines[2]}
}

// Sanitize makes untrusted text safe to show: invalid UTF-8 and control
// characters, including tabs and escape sequences, become spaces.
func Sanitize(text string, keepNewlines bool) string {
	text = strings.ToValidUTF8(text, "\uFFFD")
	return strings.Map(func(r rune) rune {
		if r == '\n' && keepNewlines {
			return r
		}

		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
}
