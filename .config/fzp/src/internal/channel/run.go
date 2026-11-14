package channel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"fzp/internal/proc"
)

var (
	// ErrBackedOut means the user left the first step with Esc.
	ErrBackedOut = errors.New("backed out of the channel")
	// ErrQuit means the user left with Ctrl+C.
	ErrQuit = errors.New("channel cancelled")
)

// Options are the per-invocation inputs that do not come from YAML.
type Options struct {
	Name   string
	Height string
	Query  string
	// Env is the environment of fzf and every command the channel runs.
	Env []string
	// HasInput reports whether the channel was opened with an input query;
	// only then does Scope run.
	HasInput bool
}

// Help for fzp's own keys.
var (
	stepHelp = []hint{{"enter", "select"}, {"esc", "back"}, {"ctrl-r", "reload"}}
	lastHelp = []hint{{"enter", "run"}, {"esc", "back"}, {"ctrl-r", "reload"}}
	askHelp  = []hint{{"enter", "choose"}, {"esc", "back"}}
	menuHelp = []hint{{"enter", "open"}, {"esc", "back"}, {"ctrl-c", "quit"}}
)

// Run shows the steps until the command succeeds, the last pick is printed,
// or the user leaves: ErrBackedOut for Esc on the first step, ErrQuit for
// Ctrl+C or when a key replaced fzf with another program.
func (s Spec) Run(ctx context.Context, o Options) error {
	scope, err := s.prepareScope(ctx, o)
	if err != nil {
		return err
	}
	defer scope.remove()

	r := runner{
		spec: s,
		opts: o,
		base: append(withoutSteps(o.Env), scope.env()...),
		show: func(ctx context.Context, sc screen) (answer, error) { return sc.show(ctx) },
	}
	return r.run(ctx)
}

type runner struct {
	spec Spec
	opts Options
	base []string
	// picks holds one row per completed step.
	picks [][]string
	// show displays a screen; tests replace it.
	show func(context.Context, screen) (answer, error)
}

// Pick shows one menu and returns the first column of the picked row.
// Esc returns ErrBackedOut; Ctrl+C returns ErrQuit.
func Pick(ctx context.Context, o Options, step Step) (string, error) {
	got, err := screen{
		step:   step,
		prompt: o.Name + "> ",
		height: o.Height,
		query:  o.Query,
		help:   menuHelp,
		env:    o.Env,
	}.show(ctx)
	if err != nil {
		return "", err
	}
	switch got.outcome {
	case picked:
		return got.rows[0][0], nil
	case back:
		return "", ErrBackedOut
	default:
		return "", ErrQuit
	}
}

func (r *runner) run(ctx context.Context) error {
	if len(r.spec.Steps) == 0 {
		again, err := r.finish(ctx, nil)
		if again {
			return ErrBackedOut
		}
		return err
	}

	queries := make([]string, len(r.spec.Steps))
	queries[0] = r.opts.Query

	for position := 0; position >= 0; {
		r.picks = r.picks[:position]

		got, err := r.show(ctx, r.screen(position, queries[position]))
		if err != nil {
			return err
		}
		queries[position] = got.query

		next, err := r.advance(ctx, position, got)
		if err != nil || next == len(r.spec.Steps) {
			return err
		}
		position = next
	}
	return ErrBackedOut
}

// advance applies an answer and returns the step to show next;
// len(steps) means the channel is done.
func (r *runner) advance(ctx context.Context, position int, got answer) (int, error) {
	switch got.outcome {
	case back:
		return position - 1, nil
	case picked:
		r.picks = append(r.picks, got.rows[0])
		if position < len(r.spec.Steps)-1 {
			return position + 1, nil
		}

		again, err := r.finish(ctx, got.rows)
		if err != nil || !again {
			return len(r.spec.Steps), err
		}
		return position, nil
	default:
		return 0, ErrQuit
	}
}

func (r *runner) screen(position int, query string) screen {
	help := stepHelp
	if position == len(r.spec.Steps)-1 && r.spec.Command != "" {
		help = lastHelp
	}

	return screen{
		step:   r.spec.Steps[position],
		prompt: "[" + r.opts.Name + "]> ",
		height: r.opts.Height,
		query:  query,
		help:   help,
		env:    r.environ(),
	}
}

// environ is the base environment plus FZP_STEP_<SELECT>_<COLUMN> for every
// completed step.
func (r *runner) environ() []string {
	env := append([]string{}, r.base...)
	for i, row := range r.picks {
		step := r.spec.Steps[i]
		if step.Select == "" {
			continue
		}

		prefix := "FZP_STEP_" + strings.ToUpper(step.Select) + "_"
		for j, column := range step.columns() {
			env = append(env, prefix+strings.ToUpper(column)+"="+row[j])
		}
	}
	return env
}

// finish runs the command after the last step. It reports again when the
// user went back, to show the last step again. Without a command, the picked
// rows are printed.
func (r *runner) finish(ctx context.Context, rows [][]string) (again bool, err error) {
	if r.spec.Command == "" {
		for _, row := range rows {
			fmt.Println(strings.Join(row, "\t"))
		}
		return false, nil
	}

	if r.spec.Confirm != "" {
		yes, err := r.ask(ctx, os.Expand(r.spec.Confirm, r.lookup), "No, go back", "Yes")
		if err != nil || !yes {
			return true, err
		}
	}

	for {
		failure := proc.Command{Argv: proc.Shell(r.spec.Command), Env: r.environ()}.Run(ctx, nil)
		if failure == nil {
			return false, nil
		}

		retry, err := r.ask(ctx, "error: "+errorText(failure), "Back", "Retry")
		if err != nil || !retry {
			return true, err
		}
	}
}

// ask shows two choices and reports whether the second was picked. Esc picks
// the first; Ctrl+C quits.
func (r *runner) ask(ctx context.Context, question, first, second string) (bool, error) {
	got, err := r.show(ctx, screen{
		step: Step{
			Title: question,
			Items: []map[string]string{{"line": first}, {"line": second}},
		},
		prompt: "[" + r.opts.Name + "]> ",
		height: r.opts.Height,
		help:   askHelp,
		env:    r.environ(),
	})
	if err != nil {
		return false, err
	}

	switch got.outcome {
	case picked:
		return got.rows[0][0] == second, nil
	case back:
		return false, nil
	default:
		return false, ErrQuit
	}
}

func (r *runner) lookup(name string) string {
	value := ""
	for _, entry := range r.environ() {
		if v, ok := strings.CutPrefix(entry, name+"="); ok {
			value = v
		}
	}
	return value
}

const maxErrorLines = 15

// errorText keeps the first line and the end of a long error.
func errorText(err error) string {
	lines := strings.Split(err.Error(), "\n")
	if len(lines) > maxErrorLines {
		lines = append([]string{lines[0], "…"}, lines[len(lines)-maxErrorLines+2:]...)
	}
	return strings.Join(lines, "\n")
}

// withoutSteps drops FZP_STEP_ values inherited from an outer channel.
func withoutSteps(environ []string) []string {
	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		if !strings.HasPrefix(entry, "FZP_STEP_") {
			kept = append(kept, entry)
		}
	}
	return kept
}
