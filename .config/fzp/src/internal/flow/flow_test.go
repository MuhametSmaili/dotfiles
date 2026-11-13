package flow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"fzp/internal/proc"
	"fzp/internal/tmux"
)

// scriptedChooser answers screens in order and records what it was shown.
type scriptedChooser struct {
	t       *testing.T
	answers []answer
	shown   []screen
}

func (s *scriptedChooser) choose(_ context.Context, sc screen) (answer, error) {
	s.shown = append(s.shown, sc)
	if len(s.answers) == 0 {
		s.t.Fatalf("unexpected screen %d: %q", len(s.shown), sc.header)
	}

	next := s.answers[0]
	s.answers = s.answers[1:]
	return next, nil
}

// recordingExecutor records the selections each run saw and returns the
// queued results, then nil.
type recordingExecutor struct {
	results []error
	runs    [][]string
}

func (r *recordingExecutor) execute(_ context.Context, _ runDef, e *env) error {
	var seen []string
	for _, entry := range e.environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "FZP_STEP_") && strings.HasSuffix(name, "_ID") {
			seen = append(seen, entry)
		}
	}
	r.runs = append(r.runs, seen)

	if len(r.results) == 0 {
		return nil
	}
	result := r.results[0]
	r.results = r.results[1:]
	return result
}

// failingSource fails a fixed number of times before listing its items.
type failingSource struct {
	failures *int
	items    []item
}

func (f failingSource) list(context.Context, *env) ([]item, error) {
	if *f.failures > 0 {
		*f.failures--
		return nil, errors.New("source broke")
	}
	return f.items, nil
}

func twoStepFlow() Def {
	return Def{
		steps: []step{
			{name: "session", title: "Session", items: []item{{ID: "$1"}, {ID: "$2"}}},
			{name: "window", title: "Window", items: []item{{ID: "@1"}, {ID: "@2"}}},
		},
		run: runDef{command: "true"},
	}
}

func newTestRunner(t *testing.T, def Def, answers ...answer) (*runner, *scriptedChooser, *recordingExecutor) {
	ui := &scriptedChooser{t: t, answers: answers}
	exec := &recordingExecutor{}
	r := &runner{
		def:     def,
		env:     newEnv(nil, tmux.Target{}),
		ui:      ui,
		execute: exec.execute,
	}
	return r, ui, exec
}

func TestFlowRunsCommandWithEverySelection(t *testing.T) {
	runner, _, exec := newTestRunner(t, twoStepFlow(),
		answer{outcome: picked, index: 1},
		answer{outcome: picked, index: 0},
	)

	err := runner.run(context.Background())

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := "FZP_STEP_SESSION_ID=$2 FZP_STEP_WINDOW_ID=@1"
	if len(exec.runs) != 1 || strings.Join(exec.runs[0], " ") != want {
		t.Errorf("runs = %v, want one run with %s", exec.runs, want)
	}
}

func TestFlowBackForgetsTheEarlierSelectionAndKeepsItsQuery(t *testing.T) {
	runner, ui, exec := newTestRunner(t, twoStepFlow(),
		answer{outcome: picked, index: 0, query: "one"},
		answer{outcome: back},
		answer{outcome: picked, index: 1, query: "one"},
		answer{outcome: picked, index: 1},
	)

	err := runner.run(context.Background())

	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if ui.shown[2].query != "one" {
		t.Errorf("query after back = %q, want one", ui.shown[2].query)
	}

	if strings.Contains(strings.Join(ui.shown[2].env, " "), "FZP_STEP_SESSION_ID") {
		t.Error("the first selection was still exported after going back to it")
	}

	want := "FZP_STEP_SESSION_ID=$2 FZP_STEP_WINDOW_ID=@2"
	if strings.Join(exec.runs[0], " ") != want {
		t.Errorf("run saw %v, want %s", exec.runs[0], want)
	}
}

func TestFlowExitResults(t *testing.T) {
	tests := []struct {
		name   string
		answer answer
		want   error
	}{
		{name: "esc on the first step backs out", answer: answer{outcome: back}, want: ErrBackedOut},
		{name: "ctrl-c quits", answer: answer{outcome: quit}, want: ErrQuit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, _, exec := newTestRunner(t, twoStepFlow(), tt.answer)

			err := runner.run(context.Background())

			if !errors.Is(err, tt.want) {
				t.Errorf("run = %v, want %v", err, tt.want)
			}
			if len(exec.runs) != 0 {
				t.Errorf("the command ran %d times", len(exec.runs))
			}
		})
	}
}

func TestFlowConfirmNoReturnsToTheLastStep(t *testing.T) {
	def := twoStepFlow()
	def.steps = def.steps[:1]
	def.run.confirm = "Remove $FZP_STEP_SESSION_ID?"
	runner, ui, exec := newTestRunner(t, def,
		answer{outcome: picked, index: 0},
		answer{outcome: picked, index: 0},
		answer{outcome: picked, index: 1},
		answer{outcome: picked, index: 1},
	)

	err := runner.run(context.Background())

	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if ui.shown[1].header != "Remove $1?" {
		t.Errorf("confirm header = %q", ui.shown[1].header)
	}

	if len(exec.runs) != 1 || exec.runs[0][0] != "FZP_STEP_SESSION_ID=$2" {
		t.Errorf("runs = %v, want one run for $2", exec.runs)
	}
}

func TestFlowFailedCommandShowsTheErrorAndRetries(t *testing.T) {
	def := twoStepFlow()
	def.steps = nil
	runner, ui, exec := newTestRunner(t, def, answer{outcome: picked, index: 0})
	exec.results = []error{&proc.Error{Name: "command", Err: errors.New("exit status 1"), Stderr: "no such pane"}}

	err := runner.run(context.Background())

	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(exec.runs) != 2 {
		t.Errorf("command ran %d times, want 2", len(exec.runs))
	}

	if !strings.Contains(ui.shown[0].header, "no such pane") {
		t.Errorf("error screen header = %q", ui.shown[0].header)
	}
}

func TestFlowRetryAfterConfirmDoesNotAskAgain(t *testing.T) {
	def := twoStepFlow()
	def.steps = nil
	def.run.confirm = "Really?"
	runner, ui, exec := newTestRunner(t, def,
		answer{outcome: picked, index: 1},
		answer{outcome: picked, index: 0},
	)
	exec.results = []error{errors.New("busy")}

	err := runner.run(context.Background())

	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(ui.shown) != 2 || len(exec.runs) != 2 {
		t.Errorf("screens = %d, runs = %d; want a confirm, an error screen and two runs", len(ui.shown), len(exec.runs))
	}
}

func TestFlowBackFromAFailedCommandWithoutStepsBacksOut(t *testing.T) {
	def := twoStepFlow()
	def.steps = nil
	runner, _, exec := newTestRunner(t, def, answer{outcome: picked, index: 1})
	exec.results = []error{errors.New("broken")}

	err := runner.run(context.Background())

	if !errors.Is(err, ErrBackedOut) {
		t.Errorf("run = %v, want ErrBackedOut", err)
	}
}

func TestFlowSourceFailureCanBeRetried(t *testing.T) {
	failures := 1
	def := twoStepFlow()
	def.steps = []step{{
		name:   "branch",
		title:  "Branch",
		items:  []item{{ID: "scratch"}},
		source: failingSource{failures: &failures, items: []item{{ID: "main"}}},
	}}
	runner, ui, exec := newTestRunner(t, def,
		answer{outcome: picked, index: 0},
		answer{outcome: picked, index: 1},
	)

	err := runner.run(context.Background())

	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(ui.shown[0].header, "Branch: source broke") {
		t.Errorf("error header = %q", ui.shown[0].header)
	}

	if len(ui.shown[1].items) != 2 || exec.runs[0][0] != "FZP_STEP_BRANCH_ID=main" {
		t.Errorf("items = %v, runs = %v", ui.shown[1].items, exec.runs)
	}
}
