// Package proc runs non-interactive shell commands for fzp.
package proc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
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

// ErrInterrupted reports a command stopped with Ctrl+C.
var ErrInterrupted = errors.New("interrupted")

// Command is one non-interactive child process. It gets no stdin and runs in
// its own process group, so stopping it stops everything it started. A nil
// Env inherits fzp's.
type Command struct {
	Argv []string
	Env  []string
}

// Shell returns the argv that runs script with bash.
func Shell(script string) []string {
	return []string{"bash", "-c", script, "fzp"}
}

// Error reports a failed command with the tail of its stderr.
type Error struct {
	Err    error
	Stderr string
}

func (e *Error) Error() string {
	if e.Stderr == "" {
		return "command: " + e.Err.Error()
	}
	return fmt.Sprintf("command: %v\n%s", e.Err, e.Stderr)
}

func (e *Error) Unwrap() error { return e.Err }

// Run runs the command with stdout going to w; a nil w discards it. Because
// the command's process group is not in the terminal's foreground, Ctrl+C
// reaches fzp, which then stops the group and returns ErrInterrupted.
func (c Command) Run(ctx context.Context, w io.Writer) error {
	interrupted, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	stderr := &tailBuffer{limit: maxStderr}

	cmd := exec.CommandContext(interrupted, c.Argv[0], c.Argv[1:]...)
	cmd.Env = c.Env
	cmd.Stdout = w
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = pipeGrace

	err := cmd.Run()

	switch {
	case err == nil:
		return nil
	case errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success():
		// The command succeeded; a background child kept a pipe open.
		return nil
	case ctx.Err() != nil:
		return ctx.Err()
	case interrupted.Err() != nil:
		return &Error{Err: ErrInterrupted, Stderr: stderr.String()}
	default:
		return &Error{Err: err, Stderr: stderr.String()}
	}
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
