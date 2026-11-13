package proc

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOutputReturnsStdout(t *testing.T) {
	cmd := Command{Argv: Shell("printf 'a\\nb'"), Timeout: time.Second, StdoutLimit: 64}

	out, err := cmd.Output(context.Background())

	if err != nil || string(out) != "a\nb" {
		t.Errorf("Output = %q, %v", out, err)
	}
}

func TestOutputFailures(t *testing.T) {
	tests := []struct {
		name   string
		script string
		limit  int
		is     error
		want   string
	}{
		{name: "exit status with stderr", script: "echo boom >&2; exit 3", limit: 64, want: "command: exit status 3\nboom"},
		{name: "output too large", script: "printf 0123456789", limit: 4, is: ErrOutputTooLarge},
		{name: "timeout", script: "sleep 5", limit: 64, is: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Command{Argv: Shell(tt.script), Timeout: 200 * time.Millisecond, StdoutLimit: tt.limit}

			_, err := cmd.Output(context.Background())

			var failure *Error
			if !errors.As(err, &failure) {
				t.Fatalf("error = %v, want *Error", err)
			}
			if tt.is != nil && !errors.Is(err, tt.is) {
				t.Errorf("error = %v, want %v", err, tt.is)
			}
			if tt.want != "" && err.Error() != tt.want {
				t.Errorf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestTimeoutStopsTheWholeProcessGroup(t *testing.T) {
	marker := t.TempDir() + "/survived"
	cmd := Command{Argv: Shell("(sleep 1; touch " + marker + ") & wait"), Timeout: 100 * time.Millisecond}

	_, err := cmd.Output(context.Background())

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want a timeout", err)
	}

	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Error("a child of the timed-out command kept running")
	}
}

func TestStderrKeepsOnlyTheTail(t *testing.T) {
	cmd := Command{Argv: Shell("head -c 20000 /dev/zero | tr '\\0' x >&2; echo END >&2; exit 1"), Timeout: time.Second}

	_, err := cmd.Output(context.Background())

	var failure *Error
	if !errors.As(err, &failure) || len(failure.Stderr) > maxStderr || !strings.HasSuffix(failure.Stderr, "END") {
		t.Errorf("stderr length %d, error %v", len(failure.Stderr), err)
	}
}
