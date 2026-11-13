package channel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// scripted answers screens in order and records what it was shown.
type scripted struct {
	t       *testing.T
	answers []answer
	shown   []screen
}

func (s *scripted) show(_ context.Context, sc screen) (answer, error) {
	s.shown = append(s.shown, sc)
	if len(s.answers) == 0 {
		s.t.Fatalf("unexpected screen %d: %q", len(s.shown), sc.header())
	}

	next := s.answers[0]
	s.answers = s.answers[1:]
	return next, nil
}

func pick(row ...string) answer {
	return answer{outcome: picked, rows: [][]string{row}}
}

// twoSteps is a channel whose command records the picks in out.
func twoSteps(t *testing.T, extra string) (Spec, string) {
	t.Helper()

	out := filepath.Join(t.TempDir(), "out")
	spec := mustLoad(t, `
steps:
  - {select: session, columns: [id, name], source: "true"}
  - {select: window, columns: [id], source: "true"}
run: echo "$FZP_STEP_SESSION_ID $FZP_STEP_SESSION_NAME $FZP_STEP_WINDOW_ID" >> `+out+`
`+extra)
	return spec, out
}

func runScripted(t *testing.T, spec Spec, answers ...answer) (*scripted, error) {
	t.Helper()

	ui := &scripted{t: t, answers: answers}
	r := runner{spec: spec, opts: Options{Name: "test"}, base: os.Environ(), show: ui.show}
	return ui, r.run(context.Background())
}

func readOut(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

func TestRunExportsEveryPickToTheCommand(t *testing.T) {
	spec, out := twoSteps(t, "")

	_, err := runScripted(t, spec, pick("$3", "work"), pick("@8"))

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := readOut(t, out); got != "$3 work @8" {
		t.Errorf("command saw %q", got)
	}
}

func TestRunBackForgetsThePickAndKeepsTheQuery(t *testing.T) {
	spec, out := twoSteps(t, "")
	first := pick("$1", "a")
	first.query = "wo"

	ui, err := runScripted(t, spec, first, answer{outcome: back}, pick("$2", "b"), pick("@1"))

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if ui.shown[2].query != "wo" {
		t.Errorf("query after back = %q, want wo", ui.shown[2].query)
	}
	if slices.ContainsFunc(ui.shown[2].env, func(e string) bool { return strings.HasPrefix(e, "FZP_STEP_SESSION_") }) {
		t.Error("the first pick was still exported after going back to it")
	}
	if got := readOut(t, out); got != "$2 b @1" {
		t.Errorf("command saw %q", got)
	}
}

func TestRunLeaving(t *testing.T) {
	tests := []struct {
		name   string
		answer answer
		want   error
	}{
		{name: "esc on the first step", answer: answer{outcome: back}, want: ErrBackedOut},
		{name: "ctrl-c", answer: answer{outcome: done}, want: ErrQuit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, out := twoSteps(t, "")

			_, err := runScripted(t, spec, tt.answer)

			if !errors.Is(err, tt.want) {
				t.Errorf("run = %v, want %v", err, tt.want)
			}
			if readOut(t, out) != "" {
				t.Error("the command ran")
			}
		})
	}
}

func TestRunConfirm(t *testing.T) {
	spec, out := twoSteps(t, `confirm: "Move $FZP_STEP_SESSION_NAME?"`)

	ui, err := runScripted(t, spec,
		pick("$1", "a"), pick("@1"),
		pick("No, go back"),
		pick("@2"),
		pick("Yes"),
	)

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.HasPrefix(ui.shown[2].header(), "Move a?") {
		t.Errorf("confirm header = %q", ui.shown[2].header())
	}
	if got := readOut(t, out); got != "$1 a @2" {
		t.Errorf("command saw %q, want one run after going back", got)
	}
}

func TestRunFailedCommandShowsTheErrorAndRetries(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "tried")
	spec := mustLoad(t, `
confirm: sure?
run: if [ -e `+marker+` ]; then exit 0; fi; touch `+marker+`; echo "no such pane" >&2; exit 1
`)

	ui, err := runScripted(t, spec, pick("Yes"), pick("Retry"))

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(ui.shown) != 2 || !strings.Contains(ui.shown[1].header(), "error: command: exit status 1\nno such pane") {
		t.Errorf("screens = %d, error header = %q", len(ui.shown), ui.shown[len(ui.shown)-1].header())
	}
}

func TestRunWithoutCommandPrintsThePick(t *testing.T) {
	spec := mustLoad(t, "steps: [{source: 'true'}]")
	stdout := captureStdout(t)

	_, err := runScripted(t, spec, answer{outcome: picked, rows: [][]string{{"a"}, {"b"}}})

	if err != nil || stdout() != "a\nb\n" {
		t.Errorf("run = %v, stdout = %q", err, stdout())
	}
}

func TestRunCommandOnlyBacksOutOfAFailure(t *testing.T) {
	spec := mustLoad(t, "run: exit 1")

	_, err := runScripted(t, spec, pick("Back"))

	if !errors.Is(err, ErrBackedOut) {
		t.Errorf("run = %v, want ErrBackedOut", err)
	}
}

// captureStdout redirects os.Stdout until the test ends and returns what was
// written so far.
func captureStdout(t *testing.T) func() string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "stdout")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	saved := os.Stdout
	os.Stdout = file
	t.Cleanup(func() {
		os.Stdout = saved
		_ = file.Close()
	})

	return func() string {
		data, _ := os.ReadFile(path)
		return string(data)
	}
}
