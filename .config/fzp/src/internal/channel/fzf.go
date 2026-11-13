package channel

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// MinFzf is the oldest fzf with every option fzp uses (--with-shell).
const MinFzf = "0.51.0"

// CheckFzf reports whether the fzf on PATH is MinFzf or newer.
func CheckFzf(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "fzf", "--version").Output()
	if err != nil {
		return fmt.Errorf("fzf: %w", err)
	}

	version, _, _ := strings.Cut(strings.TrimSpace(string(out)), " ")
	if compareVersions(version, MinFzf) < 0 {
		return fmt.Errorf("fzf %s is too old; fzp needs %s or newer", version, MinFzf)
	}
	return nil
}

// compareVersions compares dotted version numbers; missing or invalid parts
// count as 0.
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(as), len(bs)) {
		if c := cmp.Compare(versionPart(as, i), versionPart(bs, i)); c != 0 {
			return c
		}
	}
	return 0
}

func versionPart(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, _ := strconv.Atoi(parts[i])
	return n
}

// answer is what the user did on one screen.
type answer struct {
	outcome outcome
	query   string
	// rows are the picked rows, split into columns.
	rows [][]string
}

type outcome int

const (
	// done means fzf ended without a pick: Ctrl+C, or a key whose action
	// replaced fzf with another program.
	done outcome = iota
	picked
	back
)

// screen is one fzf run.
type screen struct {
	step   Step
	prompt string
	height string
	query  string
	// help describes fzp's own keys; a key the step rebinds is left out.
	help []hint
	env  []string
}

// hint is one entry of the key help in the header.
type hint struct {
	key   string
	label string
}

// show runs fzf for one step. fzf runs the list command itself (start:reload)
// so rows stream in, and stops it on exit. Enter and Esc report back through
// a result file rather than stdout, so programs started with become keep the
// terminal.
func (s screen) show(ctx context.Context) (answer, error) {
	result, err := os.CreateTemp("", "fzp-result-*")
	if err != nil {
		return answer{}, err
	}
	defer func() { _ = os.Remove(result.Name()) }()
	if err := result.Close(); err != nil {
		return answer{}, err
	}

	cmd := exec.CommandContext(ctx, "fzf", s.args()...)
	cmd.Env = append(append([]string{}, s.env...),
		"FZP_SOURCE="+s.step.producer(),
		"FZP_RESULT="+result.Name())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, os.Stdout, os.Stderr
	// SIGTERM lets fzf restore the terminal; SIGKILL would leave it raw.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }

	runErr := cmd.Run()
	if ctx.Err() != nil {
		return answer{}, ctx.Err()
	}

	data, err := os.ReadFile(result.Name())
	if err != nil {
		return answer{}, err
	}

	if len(data) == 0 {
		return answer{outcome: done}, ignoreExit(runErr)
	}
	return s.parse(string(data)), nil
}

// ignoreExit accepts fzf's abort (130) and the exit status of a program a
// key started with become; only failures to run fzf at all are errors.
func ignoreExit(err error) error {
	var exit *exec.ExitError
	if err == nil || errors.As(err, &exit) {
		return nil
	}
	return fmt.Errorf("fzf: %w", err)
}

// Result-file protocol: one line naming the outcome, the query, then one
// line per picked row. fzf shell-quotes {q} and {+}.
const (
	pickAction = `execute-silent(printf '%s\n' pick {q} {+} > "$FZP_RESULT")+abort`
	backAction = `execute-silent(printf '%s\n' back {q} > "$FZP_RESULT")+abort`
	// Enter does nothing while no row matches.
	enterGuard = `zero:unbind(enter)`
	enterAgain = `result:rebind(enter)`
	// The parenthesis form, unlike reload:..., can be followed by more
	// actions, such as a step's start key.
	reload = `reload(eval "$FZP_SOURCE")`
)

func (s screen) parse(data string) answer {
	lines := strings.Split(strings.TrimSuffix(data, "\n"), "\n")
	got := answer{query: lines[min(1, len(lines)-1)]}

	if lines[0] == "back" {
		got.outcome = back
		return got
	}

	got.outcome = picked
	columns := len(s.step.columns())
	for _, line := range lines[min(2, len(lines)):] {
		row := strings.SplitN(line, "\t", columns)
		for len(row) < columns {
			row = append(row, "")
		}
		got.rows = append(got.rows, row)
	}
	return got
}

func (s screen) args() []string {
	args := []string{
		"--height", s.height,
		"--prompt", s.prompt,
		"--header", s.header(),
		"--query", s.query,
		"--with-shell", "bash -c",
		"--delimiter", "\t",
		"--bind", s.startBinding(),
		"--bind", "ctrl-r:" + reload,
		"--bind", "enter:" + pickAction,
		"--bind", "esc:" + backAction,
		"--bind", enterGuard,
		"--bind", enterAgain,
	}

	if fields := s.step.showFields(); fields != "" {
		args = append(args, "--with-nth", fields)
	}

	if s.step.Preview != "" {
		args = append(args, "--preview", s.step.Preview, "--preview-window", cmp.Or(s.step.PreviewWindow, "right:50%"))
	}

	for _, key := range slices.Sorted(maps.Keys(s.step.Keys)) {
		if key != "start" {
			args = append(args, "--bind", key+":"+s.step.Keys[key].Do)
		}
	}
	return append(args, s.step.Fzf...)
}

// startBinding loads the rows when fzf starts, followed by the step's own
// start actions, if any.
func (s screen) startBinding() string {
	binding := "start:" + reload
	if start, ok := s.step.Keys["start"]; ok {
		binding += "+" + start.Do
	}
	return binding
}

func (s screen) header() string {
	var parts []string
	for _, builtin := range s.help {
		if _, rebound := s.step.Keys[builtin.key]; !rebound {
			parts = append(parts, builtin.key+" "+builtin.label)
		}
	}

	for _, key := range slices.Sorted(maps.Keys(s.step.Keys)) {
		if label := s.step.Keys[key].Label; label != "" {
			parts = append(parts, key+" "+label)
		}
	}

	if s.step.Title == "" {
		return strings.Join(parts, " · ")
	}
	return s.step.Title + "\n" + strings.Join(parts, " · ")
}

// showFields maps Show to fzf's 1-based field numbers.
func (s Step) showFields() string {
	columns := s.columns()
	fields := make([]string, 0, len(s.Show))
	for _, name := range s.Show {
		fields = append(fields, strconv.Itoa(slices.Index(columns, name)+1))
	}
	return strings.Join(fields, ",")
}

// producer is the shell command that lists the step's rows: the fixed items,
// then the source.
func (s Step) producer() string {
	var lines []string
	for _, item := range s.Items {
		row := make([]string, 0, len(s.columns()))
		for _, column := range s.columns() {
			row = append(row, item[column])
		}
		lines = append(lines, shellQuote(strings.Join(row, "\t")))
	}

	var commands []string
	if len(lines) > 0 {
		commands = append(commands, "printf '%s\\n' "+strings.Join(lines, " "))
	}

	if s.Source != "" {
		commands = append(commands, s.Source)
	}
	return strings.Join(commands, "\n")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
