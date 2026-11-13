package proc

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunWritesStdout(t *testing.T) {
	var out bytes.Buffer

	err := Command{Argv: Shell("printf 'a\\nb'")}.Run(context.Background(), &out)

	if err != nil || out.String() != "a\nb" {
		t.Errorf("Run = %q, %v", out.String(), err)
	}
}

func TestRunReportsExitStatusAndStderr(t *testing.T) {
	err := Command{Argv: Shell("echo boom >&2; exit 3")}.Run(context.Background(), nil)

	var failure *Error
	if !errors.As(err, &failure) || err.Error() != "command: exit status 3\nboom" {
		t.Errorf("error = %v", err)
	}
}

func TestRunKeepsOnlyTheEndOfStderr(t *testing.T) {
	err := Command{Argv: Shell("head -c 20000 /dev/zero | tr '\\0' x >&2; echo END >&2; exit 1")}.Run(context.Background(), nil)

	var failure *Error
	if !errors.As(err, &failure) || len(failure.Stderr) > maxStderr || !strings.HasSuffix(failure.Stderr, "END") {
		t.Errorf("error = %v", err)
	}
}

func TestRunSucceedsWhenABackgroundChildKeepsAPipeOpen(t *testing.T) {
	err := Command{Argv: Shell("sleep 2 & echo started")}.Run(context.Background(), &bytes.Buffer{})

	if err != nil {
		t.Errorf("Run = %v, want success", err)
	}
}

func TestCancelStopsTheWholeProcessGroup(t *testing.T) {
	pidFile := t.TempDir() + "/pid"
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- Command{Argv: Shell("sleep 30 & echo $! > " + pidFile + "; wait")}.Run(ctx, nil)
	}()
	child := waitForPID(t, pidFile)
	cancel()

	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run = %v, want context.Canceled", err)
	}
	waitUntilGone(t, child)
}

func TestInterruptStopsTheCommand(t *testing.T) {
	pidFile := t.TempDir() + "/pid"
	done := make(chan error, 1)

	go func() {
		done <- Command{Argv: Shell("echo $$ > " + pidFile + "; sleep 30")}.Run(context.Background(), nil)
	}()
	waitForPID(t, pidFile)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}

	if err := <-done; !errors.Is(err, ErrInterrupted) {
		t.Errorf("Run = %v, want ErrInterrupted", err)
	}
}

// waitForPID waits up to 5 s for a command to write its PID to path.
func waitForPID(t *testing.T, path string) int {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if pid, convErr := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && convErr == nil {
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no PID in %s", path)
	return 0
}

// waitUntilGone waits up to 5 s for pid to exit.
func waitUntilGone(t *testing.T, pid int) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("process %d still runs", pid)
}
