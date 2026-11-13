// Package flow runs flow channels: zero or more selection steps followed by
// one command. Each step is one fzf screen; Esc goes back a step, and the
// selected items are exported to later steps and the command.
package flow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"fzp/internal/proc"
	"fzp/internal/tmux"
)

var (
	// ErrBackedOut means the user left the first screen with Esc.
	ErrBackedOut = errors.New("backed out of the flow")
	// ErrQuit means the user cancelled the flow with Ctrl+C.
	ErrQuit = errors.New("flow cancelled")
)

// Options are the per-invocation inputs that do not come from YAML.
type Options struct {
	Channel string
	Height  string
	Target  tmux.Target
	// Environ is the base environment of every command the flow starts.
	Environ []string
}

// Run walks the flow until its command succeeds, the user backs out of the
// first step (ErrBackedOut) or cancels (ErrQuit).
func (d Def) Run(ctx context.Context, o Options) error {
	r := runner{
		def:     d,
		env:     newEnv(o.Environ, o.Target),
		ui:      fzfChooser{height: o.Height, prompt: "[" + o.Channel + "]> "},
		execute: execute,
	}
	return r.run(ctx)
}

// outcome is what the user did on a screen.
type outcome int

const (
	picked outcome = iota
	back
	reload
	quit
)

// screen is one list shown to the user. Previews run in dir.
type screen struct {
	header  string
	items   []item
	query   string
	preview Preview
	env     []string
	dir     string
}

type answer struct {
	outcome outcome
	index   int
	query   string
}

// chooser shows a screen and reports the user's answer.
type chooser interface {
	choose(ctx context.Context, s screen) (answer, error)
}

// executor runs the flow's final command.
type executor func(ctx context.Context, run runDef, env *env) error

type runner struct {
	def     Def
	env     *env
	ui      chooser
	execute executor
}

const stepHelp = "enter select · esc back · ctrl-r reload · ctrl-c quit"

func (r *runner) run(ctx context.Context) error {
	queries := make([]string, len(r.def.steps))
	position := 0

	for position <= len(r.def.steps) {
		if position < 0 {
			return ErrBackedOut
		}

		r.env.truncate(position)

		var move int
		var err error
		if position == len(r.def.steps) {
			move, err = r.finish(ctx)
		} else {
			move, err = r.step(ctx, position, &queries[position])
		}

		if err != nil {
			return err
		}
		position += move
	}
	return nil
}

// step shows one selection screen and returns how far to move: +1 after a
// pick, -1 for back, 0 to show the step again.
func (r *runner) step(ctx context.Context, position int, query *string) (int, error) {
	current := r.def.steps[position]

	items, err := r.load(ctx, current)
	if err != nil {
		return r.recover(ctx, fmt.Errorf("%s: %w", current.title, err))
	}

	got, err := r.ui.choose(ctx, screen{
		header:  current.title + "\n" + stepHelp,
		items:   items,
		query:   *query,
		preview: current.preview,
		env:     r.env.environ(),
		dir:     r.env.target.Dir(),
	})
	if err != nil {
		return 0, err
	}
	*query = got.query

	switch got.outcome {
	case picked:
		r.env.push(current.name, items[got.index])
		return 1, nil
	case back:
		return -1, nil
	case reload:
		return 0, nil
	default:
		return 0, ErrQuit
	}
}

func (r *runner) load(ctx context.Context, current step) ([]item, error) {
	items := append([]item(nil), current.items...)
	if current.source == nil {
		return items, nil
	}

	listed, err := current.source.list(ctx, r.env)
	if err != nil {
		return nil, err
	}
	return append(items, listed...), nil
}

// finish confirms once, then runs the command until it succeeds (1) or the
// user goes back to the last step (-1). Retrying a failed command does not
// ask for confirmation again.
func (r *runner) finish(ctx context.Context) (int, error) {
	if r.def.run.confirm != "" {
		move, err := r.confirm(ctx)
		if err != nil || move != 0 {
			return move, err
		}
	}

	for {
		failure := r.execute(ctx, r.def.run, r.env)
		if failure == nil {
			return 1, nil
		}

		move, err := r.recover(ctx, failure)
		if err != nil || move != 0 {
			return move, err
		}
	}
}

// confirm returns 0 to proceed and -1 to go back.
func (r *runner) confirm(ctx context.Context) (int, error) {
	message, err := r.env.expand(r.def.run.confirm)
	if err != nil {
		return r.recover(ctx, fmt.Errorf("confirm: %w", err))
	}

	for {
		got, err := r.ui.choose(ctx, screen{
			header: message,
			items:  []item{{ID: "no", Label: "No, go back"}, {ID: "yes", Label: "Yes"}},
			env:    r.env.environ(),
		})
		if err != nil {
			return 0, err
		}

		switch {
		case got.outcome == quit:
			return 0, ErrQuit
		case got.outcome == picked && got.index == 1:
			return 0, nil
		case got.outcome != reload:
			return -1, nil
		}
	}
}

// recover shows a failure and lets the user retry (0), go back (-1) or quit.
func (r *runner) recover(ctx context.Context, failure error) (int, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	got, err := r.ui.choose(ctx, screen{
		header: errorHeader(failure),
		items:  []item{{ID: "retry", Label: "Retry"}, {ID: "back", Label: "Back"}},
		env:    r.env.environ(),
	})
	if err != nil {
		return 0, err
	}

	switch {
	case got.outcome == quit:
		return 0, ErrQuit
	case got.outcome == picked && got.index == 0, got.outcome == reload:
		return 0, nil
	default:
		return -1, nil
	}
}

const maxErrorLines = 15

func errorHeader(err error) string {
	lines := strings.Split(err.Error(), "\n")
	if len(lines) > maxErrorLines {
		lines = append([]string{lines[0], "…"}, lines[len(lines)-maxErrorLines+2:]...)
	}
	return "error: " + strings.Join(lines, "\n")
}

// execute runs the final command in the target pane's directory. An
// interactive command owns the terminal and has no timeout; any other command
// gets no stdin, and its stderr is kept so a failure can be shown.
func execute(ctx context.Context, run runDef, env *env) error {
	if !run.interactive {
		_, err := proc.Command{
			Argv:    proc.Shell(run.command),
			Dir:     env.target.Dir(),
			Env:     env.environ(),
			Timeout: run.timeout,
		}.Output(ctx)
		return err
	}

	argv := proc.Shell(run.command)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = env.target.Dir()
	cmd.Env = env.environ()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }

	if err := cmd.Run(); err != nil {
		return &proc.Error{Name: "command", Err: err}
	}
	return nil
}
