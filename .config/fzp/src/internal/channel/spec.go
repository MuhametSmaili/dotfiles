// Package channel runs channels: one or more fzf steps, optionally followed
// by a shell command.
//
// Each step lists rows from fixed items and a shell command. Rows are
// tab-separated columns; the columns of the row picked in a step named NAME
// reach later steps and the command as FZP_STEP_NAME_<COLUMN>. Keys bind
// plain fzf actions, so another channel opens with become:fzp ..., and a
// query is handed over with become:fzp --input-query {q} OTHER.
package channel

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"fzp/internal/catalog"
)

// Spec is a channel's YAML.
type Spec struct {
	Metadata catalog.Metadata `yaml:"metadata"`
	// Scope runs once when the channel is opened with an input query; its
	// output is kept in FZP_SCOPE_FILE until the channel closes.
	Scope string `yaml:"scope"`
	Steps []Step `yaml:"steps"`
	// Command runs after the last step; without it, the picked rows are
	// printed to stdout.
	Command string `yaml:"run"`
	// Confirm asks before Command; $VARIABLES are expanded.
	Confirm string `yaml:"confirm"`
}

// Step is one fzf screen.
type Step struct {
	// Select names the step; its pick is exported as FZP_STEP_<SELECT>_*.
	Select string `yaml:"select"`
	Title  string `yaml:"title"`
	// Items are fixed rows listed before the source's rows, keyed by column.
	Items  []map[string]string `yaml:"items"`
	Source string              `yaml:"source"`
	// Columns name the tab-separated fields of each row; the default is one
	// column, "line".
	Columns []string `yaml:"columns"`
	// Show lists the columns to display and search; the default is all.
	Show          []string `yaml:"show"`
	Preview       string   `yaml:"preview"`
	PreviewWindow string   `yaml:"preview_window"`
	// Keys map fzf keys and events to actions. A start entry runs after
	// fzp's own start action, which loads the rows.
	Keys map[string]Key `yaml:"keys"`
	// Fzf holds extra fzf options, such as --ansi or --multi.
	Fzf []string `yaml:"fzf"`
}

// Key binds a key to fzf actions. A key with a label is listed in the
// header.
type Key struct {
	Do    string `yaml:"do"`
	Label string `yaml:"label"`
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// Load reads and validates the channel's YAML.
func Load(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, fmt.Errorf("%s: %w", path, err)
	}

	if err := spec.validate(); err != nil {
		return Spec{}, fmt.Errorf("%s: %w", path, err)
	}
	return spec, nil
}

func (s Spec) validate() error {
	if len(s.Steps) == 0 && s.Command == "" {
		return errors.New("steps or run is required")
	}

	if s.Confirm != "" && s.Command == "" {
		return errors.New("confirm needs run")
	}

	seen := map[string]bool{}
	for i, step := range s.Steps {
		if err := step.validate(); err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}

		if step.Select != "" && seen[step.Select] {
			return fmt.Errorf("step %d: duplicate select %q", i+1, step.Select)
		}
		seen[step.Select] = true
	}
	return nil
}

func (s Step) validate() error {
	if s.Select != "" && !namePattern.MatchString(s.Select) {
		return fmt.Errorf("select %q must match %s", s.Select, namePattern)
	}

	if s.Source == "" && len(s.Items) == 0 {
		return errors.New("source or items is required")
	}

	columns := s.columns()
	if err := checkNames("column", columns); err != nil {
		return err
	}

	for _, name := range s.Show {
		if !slices.Contains(columns, name) {
			return fmt.Errorf("show: unknown column %q", name)
		}
	}

	for i, item := range s.Items {
		if err := checkItem(item, columns); err != nil {
			return fmt.Errorf("item %d: %w", i+1, err)
		}
	}

	for key, binding := range s.Keys {
		if strings.TrimSpace(binding.Do) == "" {
			return fmt.Errorf("key %s: do is required", key)
		}
	}
	return nil
}

func checkNames(kind string, names []string) error {
	seen := map[string]bool{}
	for _, name := range names {
		if !namePattern.MatchString(name) {
			return fmt.Errorf("%s %q must match %s", kind, name, namePattern)
		}

		if seen[name] {
			return fmt.Errorf("duplicate %s %q", kind, name)
		}
		seen[name] = true
	}
	return nil
}

func checkItem(item map[string]string, columns []string) error {
	for name, value := range item {
		if !slices.Contains(columns, name) {
			return fmt.Errorf("unknown column %q", name)
		}

		if strings.ContainsAny(value, "\t\n") {
			return fmt.Errorf("column %q must not contain tabs or newlines", name)
		}
	}
	return nil
}

func (s Step) columns() []string {
	if len(s.Columns) == 0 {
		return []string{"line"}
	}
	return s.Columns
}
