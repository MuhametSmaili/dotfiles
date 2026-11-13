package flow

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"fzp/internal/catalog"
)

const (
	defaultSourceTimeout = 30 * time.Second
	defaultRunTimeout    = 10 * time.Minute
	maxTimeout           = 30 * time.Minute
)

// spec is the YAML form of a flow.
type spec struct {
	Metadata catalog.Metadata `yaml:"metadata"`
	Steps    []stepSpec       `yaml:"steps"`
	Run      runSpec          `yaml:"run"`
}

type stepSpec struct {
	Select  string      `yaml:"select"`
	Title   string      `yaml:"title"`
	Items   []itemSpec  `yaml:"items"`
	Source  *sourceSpec `yaml:"source"`
	Preview Preview     `yaml:"preview"`
}

// Preview is a step's preview command; {id} is the highlighted item's ID.
type Preview struct {
	Command string `yaml:"command"`
	Window  string `yaml:"window"`
}

type sourceSpec struct {
	Builtin string    `yaml:"builtin"`
	With    yaml.Node `yaml:"with"`

	Command     string   `yaml:"command"`
	Columns     []string `yaml:"columns"`
	Label       []string `yaml:"label"`
	Description string   `yaml:"description"`
	Timeout     duration `yaml:"timeout"`
}

type runSpec struct {
	Command     string   `yaml:"command"`
	Confirm     string   `yaml:"confirm"`
	Interactive bool     `yaml:"interactive"`
	Timeout     duration `yaml:"timeout"`
}

// itemSpec is a fixed choice. id, label and description are reserved; every
// other key becomes a field of the item.
type itemSpec item

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

func validName(name string) bool {
	return namePattern.MatchString(name)
}

// duration is a positive time.Duration written as a Go duration string.
type duration time.Duration

func (d *duration) UnmarshalYAML(node *yaml.Node) error {
	parsed, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("line %d: invalid duration %q", node.Line, node.Value)
	}

	if parsed <= 0 || parsed > maxTimeout {
		return fmt.Errorf("line %d: duration %s must be positive and at most %s", node.Line, parsed, maxTimeout)
	}

	*d = duration(parsed)
	return nil
}

func (d duration) or(fallback time.Duration) time.Duration {
	if d == 0 {
		return fallback
	}
	return time.Duration(d)
}

func (s *itemSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: an item must be a mapping", node.Line)
	}

	s.Fields = map[string]string{}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if value.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: item field %q must be a scalar", value.Line, key.Value)
		}

		if err := s.set(key.Value, value.Value); err != nil {
			return fmt.Errorf("line %d: %w", key.Line, err)
		}
	}
	return nil
}

func (s *itemSpec) set(key, value string) error {
	switch key {
	case "id":
		s.ID = value
	case "label":
		s.Label = value
	case "description":
		s.Description = value
	default:
		if !validName(key) {
			return fmt.Errorf("item field %q must match %s", key, namePattern)
		}
		s.Fields[key] = value
	}
	return nil
}

// Def is a validated flow, ready to run.
type Def struct {
	Metadata catalog.Metadata
	steps    []step
	run      runDef
}

type step struct {
	name    string
	title   string
	items   []item
	source  source
	preview Preview
}

type runDef struct {
	command     string
	confirm     string
	interactive bool
	timeout     time.Duration
}

// Load reads and validates the flow channel.
func Load(channel catalog.Channel) (Def, error) {
	data, err := os.ReadFile(channel.Path)
	if err != nil {
		return Def{}, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var s spec
	if err := decoder.Decode(&s); err != nil {
		return Def{}, fmt.Errorf("%s: %w", channel.Path, err)
	}

	def, err := compile(s)
	if err != nil {
		return Def{}, fmt.Errorf("%s: %w", channel.Path, err)
	}
	return def, nil
}

func compile(s spec) (Def, error) {
	if strings.TrimSpace(s.Run.Command) == "" {
		return Def{}, errors.New("run.command is required")
	}

	if s.Run.Interactive && s.Run.Timeout != 0 {
		return Def{}, errors.New("run.timeout does not apply to interactive commands")
	}

	steps, err := compileSteps(s.Steps)
	if err != nil {
		return Def{}, err
	}

	def := Def{
		Metadata: s.Metadata,
		steps:    steps,
		run: runDef{
			command:     s.Run.Command,
			confirm:     s.Run.Confirm,
			interactive: s.Run.Interactive,
			timeout:     s.Run.Timeout.or(defaultRunTimeout),
		},
	}
	return def, nil
}

func compileSteps(specs []stepSpec) ([]step, error) {
	steps := make([]step, 0, len(specs))
	seen := map[string]bool{}

	for i, spec := range specs {
		compiled, err := compileStep(spec)
		if err != nil {
			return nil, fmt.Errorf("step %d (%s): %w", i+1, spec.Select, err)
		}

		if seen[compiled.name] {
			return nil, fmt.Errorf("step %d: duplicate step name %q", i+1, compiled.name)
		}
		seen[compiled.name] = true

		steps = append(steps, compiled)
	}
	return steps, nil
}

func compileStep(spec stepSpec) (step, error) {
	if !validName(spec.Select) {
		return step{}, fmt.Errorf("select must match %s", namePattern)
	}

	if len(spec.Items) == 0 && spec.Source == nil {
		return step{}, errors.New("a step needs items, a source, or both")
	}

	items, err := compileItems(spec.Items)
	if err != nil {
		return step{}, err
	}

	compiled := step{
		name:    spec.Select,
		title:   firstNonEmpty(spec.Title, spec.Select),
		items:   items,
		preview: spec.Preview,
	}

	if spec.Source != nil {
		compiled.source, err = compileSource(spec.Source)
		if err != nil {
			return step{}, fmt.Errorf("source: %w", err)
		}
	}
	return compiled, nil
}

func compileItems(specs []itemSpec) ([]item, error) {
	items := make([]item, 0, len(specs))
	seen := map[string]bool{}

	for i, spec := range specs {
		if spec.ID == "" {
			return nil, fmt.Errorf("item %d: id is required", i+1)
		}

		if seen[spec.ID] {
			return nil, fmt.Errorf("item %d: duplicate id %q", i+1, spec.ID)
		}
		seen[spec.ID] = true

		spec.Label = firstNonEmpty(spec.Label, spec.ID)
		items = append(items, item(spec))
	}
	return items, nil
}

func compileSource(spec *sourceSpec) (source, error) {
	switch {
	case spec.Builtin != "" && spec.Command != "":
		return nil, errors.New("set builtin or command, not both")
	case spec.Builtin != "":
		return compileBuiltin(spec)
	case spec.Command != "":
		return compileCommand(spec)
	default:
		return nil, errors.New("set builtin or command")
	}
}

func compileBuiltin(spec *sourceSpec) (source, error) {
	if len(spec.Columns) > 0 || len(spec.Label) > 0 || spec.Description != "" || spec.Timeout != 0 {
		return nil, errors.New("columns, label, description and timeout apply to command sources only")
	}

	factory, ok := builtins[spec.Builtin]
	if !ok {
		return nil, fmt.Errorf("unknown builtin %q (known: %s)", spec.Builtin, strings.Join(builtinNames(), ", "))
	}

	built, err := factory(&spec.With)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.Builtin, err)
	}
	return built, nil
}

func compileCommand(spec *sourceSpec) (source, error) {
	if spec.With.Kind != 0 {
		return nil, errors.New("with applies to builtin sources only")
	}

	if err := checkColumns(spec.Columns, spec.Label, spec.Description); err != nil {
		return nil, err
	}

	compiled := commandSource{
		command:     spec.Command,
		columns:     spec.Columns,
		label:       spec.Label,
		description: spec.Description,
		timeout:     spec.Timeout.or(defaultSourceTimeout),
	}
	return compiled, nil
}

func checkColumns(columns, label []string, description string) error {
	known := make(map[string]bool, len(columns))
	for _, column := range columns {
		if !validName(column) {
			return fmt.Errorf("column %q must match %s", column, namePattern)
		}

		if known[column] {
			return fmt.Errorf("duplicate column %q", column)
		}
		known[column] = true
	}

	if len(known) == 0 && (len(label) > 0 || description != "") {
		return errors.New("label and description need columns")
	}

	for _, column := range append(append([]string{}, label...), nonEmpty(description)...) {
		if !known[column] {
			return fmt.Errorf("unknown column %q", column)
		}
	}
	return nil
}

// decodeParams decodes a builtin's with: block, rejecting unknown keys.
func decodeParams(node *yaml.Node, out any) error {
	if node.Kind == 0 {
		return nil
	}

	data, err := yaml.Marshal(node)
	if err != nil {
		return err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("with: %w", err)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
