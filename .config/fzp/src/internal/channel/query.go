package channel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"fzp/internal/proc"
)

// Filter writes the rows of the channel's first step that match query, as
// that step would show them: it runs the step's list and filters it with
// fzf --filter and the step's matching options.
func (s Spec) Filter(ctx context.Context, o Options, query string, w io.Writer) error {
	step := s.Steps[0]

	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}

	args := []string{"--filter", query, "--delimiter", "\t"}
	if fields := step.showFields(); fields != "" {
		args = append(args, "--with-nth", fields)
	}

	filter := exec.CommandContext(ctx, "fzf", append(args, step.Fzf...)...)
	filter.Env = o.Env
	filter.Stdin, filter.Stdout, filter.Stderr = reader, w, os.Stderr

	// Closing fzp's copies of the pipe ends cannot lose data: fzf holds the
	// read end, and the list is complete before ours is closed.
	startErr := filter.Start()
	_ = reader.Close()
	if startErr != nil {
		_ = writer.Close()
		return fmt.Errorf("start fzf: %w", startErr)
	}

	listErr := proc.Command{Argv: proc.Shell(step.producer()), Env: o.Env}.Run(ctx, writer)
	_ = writer.Close()

	filterErr := filter.Wait()
	if listErr != nil {
		return fmt.Errorf("source: %w", listErr)
	}

	// fzf --filter exits 1 when nothing matches.
	var exit *exec.ExitError
	if errors.As(filterErr, &exit) && exit.ExitCode() == 1 {
		return nil
	}
	return filterErr
}

// scope is the output of the channel's scope command, kept in a file fzp
// owns for as long as the channel is open. A zero scope has no file.
type scope struct {
	path string
}

// prepareScope runs the scope command once when the channel was opened with
// an input query, so that reloads read FZP_SCOPE_FILE instead of computing
// it again on every keystroke.
func (s Spec) prepareScope(ctx context.Context, o Options) (scope, error) {
	if !o.HasInput || s.Scope == "" {
		return scope{}, nil
	}

	file, err := os.CreateTemp("", "fzp-scope-*")
	if err != nil {
		return scope{}, err
	}

	runErr := proc.Command{Argv: proc.Shell(s.Scope), Env: o.Env}.Run(ctx, file)
	closeErr := file.Close()
	if err := errors.Join(runErr, closeErr); err != nil {
		_ = os.Remove(file.Name())
		return scope{}, fmt.Errorf("scope: %w", err)
	}
	return scope{path: file.Name()}, nil
}

func (s scope) env() []string {
	if s.path == "" {
		return nil
	}
	return []string{"FZP_SCOPE_FILE=" + s.path}
}

func (s scope) remove() {
	if s.path != "" {
		_ = os.Remove(s.path)
	}
}
