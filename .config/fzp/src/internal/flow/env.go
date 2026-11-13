package flow

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"fzp/internal/tmux"
)

// item is one choice on a step screen. Its ID, label and fields are exported
// to later steps as FZP_STEP_<STEP>_ID, _LABEL and _<FIELD>.
type item struct {
	ID          string
	Label       string
	Description string
	Fields      map[string]string
}

// env is the environment of every command a flow starts: fzp's environment
// without inherited FZP_STEP_ values, the target, and the selections so far.
type env struct {
	base   []string
	target tmux.Target
	chosen []choice
}

type choice struct {
	step string
	item item
}

func newEnv(environ []string, target tmux.Target) *env {
	base := make([]string, 0, len(environ))
	for _, entry := range environ {
		if !strings.HasPrefix(entry, "FZP_STEP_") {
			base = append(base, entry)
		}
	}
	return &env{base: base, target: target}
}

func (e *env) environ() []string {
	out := append(append([]string{}, e.base...), e.target.Environ()...)
	for _, c := range e.chosen {
		out = append(out, stepVars(c.step, c.item)...)
	}
	return out
}

func stepVars(step string, chosen item) []string {
	prefix := "FZP_STEP_" + strings.ToUpper(step) + "_"
	vars := []string{prefix + "ID=" + chosen.ID, prefix + "LABEL=" + chosen.Label}

	names := make([]string, 0, len(chosen.Fields))
	for name := range chosen.Fields {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		vars = append(vars, prefix+strings.ToUpper(name)+"="+chosen.Fields[name])
	}
	return vars
}

// lookup finds a variable. Later entries win, as in exec's environment.
func (e *env) lookup(name string) (string, bool) {
	vars := e.environ()
	for i := len(vars) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(vars[i], name+"="); ok {
			return value, true
		}
	}
	return "", false
}

// expand replaces $NAME and ${NAME} in text. Unset variables are an error,
// so that a typo does not silently become an empty argument.
func (e *env) expand(text string) (string, error) {
	var missing []string
	expanded := os.Expand(text, func(name string) string {
		value, ok := e.lookup(name)
		if !ok {
			missing = append(missing, name)
		}
		return value
	})

	if len(missing) > 0 {
		return "", fmt.Errorf("unset variable %s in %q", strings.Join(missing, ", "), text)
	}
	return expanded, nil
}

func (e *env) push(step string, chosen item) {
	e.chosen = append(e.chosen, choice{step: step, item: chosen})
}

// truncate keeps the selections of the first n steps.
func (e *env) truncate(n int) {
	e.chosen = e.chosen[:n]
}
