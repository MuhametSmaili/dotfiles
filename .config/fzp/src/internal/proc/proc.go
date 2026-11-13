// Package proc runs bounded, non-interactive child processes.
package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const (
	maxStderr = 8 << 10
	// pipeGrace bounds how long a finished command's background children may
	// keep its output pipes open.
	pipeGrace = 200 * time.Millisecond
)

// ErrOutputTooLarge reports stdout beyond Command.StdoutLimit.
var ErrOutputTooLarge = errors.New("output exceeds limit")

// Command is one non-interactive child process. It gets no stdin and runs in
// its own process group, so a timeout or cancellation stops everything it
// started. A zero StdoutLimit discards stdout; a nil Env inherits fzp's.
type Command struct {
	Argv        []string
	Dir         string
	Env         []string
	Timeout     time.Duration
	StdoutLimit int
}

// Shell returns the argv that runs script with bash.
func Shell(script string) []string {
	return []string{"bash", "-c", script, "fzp"}
}

// Error reports a failed command with the tail of its stderr.
type Error struct {
	Name   string
	Err    error
	Stderr string
}

func (e *Error) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("%s: %v", e.Name, e.Err)
	}
	return fmt.Sprintf("%s: %v\n%s", e.Name, e.Err, e.Stderr)
}

func (e *Error) Unwrap() error { return e.Err }

// Output runs the command and returns its stdout.
func (c Command) Output(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	stdout := &limitedBuffer{limit: c.StdoutLimit}
	stderr := &tailBuffer{limit: maxStderr}

	cmd := c.build(ctx)
	cmd.Stdout = stdout
	if c.StdoutLimit == 0 {
		cmd.Stdout = io.Discard
	}
	cmd.Stderr = stderr

	err := cmd.Run()
	if err == nil && stdout.overflow {
		err = fmt.Errorf("%w of %d bytes", ErrOutputTooLarge, c.StdoutLimit)
	}

	if err != nil {
		return nil, c.failure(ctx, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func (c Command) build(ctx context.Context) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
	cmd.Dir = c.Dir
	cmd.Env = c.Env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = pipeGrace
	return cmd
}

// failure explains why the command failed; a timeout or cancellation is
// reported instead of the resulting "signal: killed".
func (c Command) failure(ctx context.Context, err error, stderr string) error {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		err = fmt.Errorf("timed out after %s: %w", c.Timeout, context.DeadlineExceeded)
	case ctx.Err() != nil:
		err = ctx.Err()
	}
	return &Error{Name: c.name(), Err: err, Stderr: stderr}
}

func (c Command) name() string {
	if len(c.Argv) >= 3 && c.Argv[0] == "bash" && c.Argv[1] == "-c" {
		return "command"
	}
	return c.Argv[0]
}

// limitedBuffer keeps at most limit bytes and records whether more arrived.
// The buffer is a named field, not embedded: an embedded bytes.Buffer would
// promote ReadFrom, which io.Copy prefers over Write, bypassing the limit.
type limitedBuffer struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	room := b.limit - b.buf.Len()
	if len(p) > room {
		b.overflow = true
		b.buf.Write(p[:max(room, 0)])
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

// tailBuffer keeps the last limit bytes written to it.
type tailBuffer struct {
	data  []byte
	limit int
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	if extra := len(b.data) - b.limit; extra > 0 {
		b.data = b.data[extra:]
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	return strings.TrimSpace(string(b.data))
}
