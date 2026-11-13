// Package screen runs single-screen channels: one fzf list whose source,
// preview and key bindings come from YAML.
package screen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"

	"fzp/internal/catalog"
)

// Spec is the YAML form of a screen channel.
type Spec struct {
	Metadata   catalog.Metadata  `yaml:"metadata"`
	Source     Source            `yaml:"source"`
	Preview    Preview           `yaml:"preview"`
	UI         UI                `yaml:"ui"`
	Navigation map[string]Jump   `yaml:"navigation"`
	Actions    map[string]Action `yaml:"actions"`
	FzfOptions []string          `yaml:"fzf_options"`
}

// Script is a command given as a shell string or as a script path relative
// to the channel directory plus a shell-syntax argument string.
type Script struct {
	Command string `yaml:"command"`
	Script  string `yaml:"script"`
	Args    string `yaml:"args"`
}

// Source produces the list.
type Source struct {
	Script    `yaml:",inline"`
	Delimiter string `yaml:"delimiter"`
	ANSI      bool   `yaml:"ansi"`
	NoSort    bool   `yaml:"no_sort"`
}

// Preview describes the preview window.
type Preview struct {
	Script `yaml:",inline"`
	Window string `yaml:"window"`
}

// UI holds the prompt, header and height. Multi selection is on by default.
type UI struct {
	Header string `yaml:"header"`
	Prompt string `yaml:"prompt"`
	Height string `yaml:"height"`
	Multi  *bool  `yaml:"multi"`
}

// Jump is a navigation key that switches to another channel.
type Jump struct {
	Channel string `yaml:"channel"`
	Label   string `yaml:"label"`
}

// Action is a key binding. Mode is an fzf action that takes a command
// (execute, execute-silent, become, reload), or raw for a literal fzf
// action list in command.
type Action struct {
	Script `yaml:",inline"`
	Mode   string `yaml:"mode"`
	Label  string `yaml:"label"`
}

var commandModes = map[string]bool{"execute": true, "execute-silent": true, "become": true, "reload": true}

// Load reads and validates the channel's YAML.
func Load(channel catalog.Channel) (Spec, error) {
	data, err := os.ReadFile(channel.Path)
	if err != nil {
		return Spec{}, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, fmt.Errorf("%s: %w", channel.Path, err)
	}

	if err := spec.validate(); err != nil {
		return Spec{}, fmt.Errorf("%s: %w", channel.Path, err)
	}
	return spec, nil
}

func (s Spec) validate() error {
	if err := s.Source.validate("source", true); err != nil {
		return err
	}

	if err := s.Preview.validate("preview", false); err != nil {
		return err
	}

	for _, key := range sortedKeys(s.Navigation) {
		if s.Navigation[key].Channel == "" {
			return fmt.Errorf("navigation %s: channel is required", key)
		}
	}

	for _, key := range sortedKeys(s.Actions) {
		if err := s.Actions[key].validate(); err != nil {
			return fmt.Errorf("action %s: %w", key, err)
		}
	}
	return nil
}

func (s Script) validate(name string, required bool) error {
	switch {
	case s.Command != "" && s.Script != "":
		return fmt.Errorf("%s: set command or script, not both", name)
	case s.Command != "" && s.Args != "":
		return fmt.Errorf("%s: args applies to script only", name)
	case required && s.Command == "" && s.Script == "":
		return fmt.Errorf("%s: command or script is required", name)
	default:
		return nil
	}
}

func (a Action) validate() error {
	if !commandModes[a.Mode] && a.Mode != "raw" {
		return fmt.Errorf("invalid mode %q (use execute, execute-silent, become, reload or raw)", a.Mode)
	}

	if a.Mode == "raw" && a.Script.Script != "" {
		return errors.New("raw actions take a command, not a script")
	}
	return a.Script.validate("action", true)
}

// shell returns the command as a shell string. A script path is resolved
// against dir and quoted; defaultArgs applies when args is empty.
func (s Script) shell(dir, defaultArgs string) string {
	if s.Script == "" {
		return s.Command
	}

	path := s.Script
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}

	args := s.Args
	if args == "" {
		args = defaultArgs
	}
	return strings.TrimSpace(shellQuote(path) + " " + args)
}

// Options are the per-invocation inputs that do not come from YAML.
type Options struct {
	Channel string
	// Dir is the channel directory; relative script paths resolve from it.
	Dir    string
	Query  string
	Height string
	// Self is the fzp executable, used by navigation keys.
	Self string
	Env  []string
}

// Args returns the fzf arguments for the channel. Commands fzf runs itself,
// such as previews and actions, use bash like every other fzp command.
func (s Spec) Args(o Options) []string {
	b := &builder{help: []string{"ctrl-s:channels"}}

	b.arg("--with-shell", "bash -c")
	b.arg("--height", firstNonEmpty(s.UI.Height, o.Height))
	b.arg("--prompt", firstNonEmpty(s.UI.Prompt, "["+o.Channel+"]> "))
	b.arg("--query", o.Query)
	b.arg("--preview", firstNonEmpty(s.Preview.shell(o.Dir, "{}"), "echo {}"))
	b.arg("--preview-window", firstNonEmpty(s.Preview.Window, "right:50%"))
	b.arg("--bind", "ctrl-s:become:"+shellQuote(o.Self))
	b.flags(s)

	for _, key := range sortedKeys(s.Navigation) {
		jump := s.Navigation[key]
		b.bind(key, "become:"+shellQuote(o.Self)+" "+shellQuote(jump.Channel), firstNonEmpty(jump.Label, jump.Channel))
	}

	for _, key := range sortedKeys(s.Actions) {
		action := s.Actions[key]
		b.bind(key, action.binding(o.Dir), firstNonEmpty(action.Label, key))
	}

	b.arg("--header", strings.Join(append(nonEmpty(s.UI.Header), b.help...), " | "))
	b.arg(s.FzfOptions...)
	return b.args
}

func (a Action) binding(dir string) string {
	command := a.shell(dir, "{}")
	if a.Mode == "raw" {
		return command
	}
	return a.Mode + "(" + command + ")"
}

type builder struct {
	args []string
	help []string
}

func (b *builder) arg(values ...string) {
	b.args = append(b.args, values...)
}

func (b *builder) bind(key, action, label string) {
	b.arg("--bind", key+":"+action)
	b.help = append(b.help, key+":"+label)
}

func (b *builder) flags(s Spec) {
	if s.UI.Multi == nil || *s.UI.Multi {
		b.arg("--multi")
	}

	if s.Source.Delimiter != "" {
		b.arg("--delimiter", s.Source.Delimiter)
	}

	if s.Source.ANSI {
		b.arg("--ansi")
	}

	if s.Source.NoSort {
		b.arg("--no-sort")
	}
}

// Run shows the channel and waits for fzf. fzf starts the source as its
// default command, with --with-shell, and stops it when the user is done, so
// a long listing never outlives the picker. Leaving with Esc, Ctrl+C or no
// match is not an error.
func (s Spec) Run(ctx context.Context, o Options) error {
	cmd := exec.CommandContext(ctx, "fzf", s.Args(o)...)
	cmd.Env = append(append([]string{}, o.Env...), "FZF_DEFAULT_COMMAND="+s.Source.shell(o.Dir, ""))
	// fzf only runs the default command when stdin is a terminal.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// SIGTERM lets fzf restore the terminal; SIGKILL would leave it raw.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }

	err := cmd.Run()

	var exit *exec.ExitError
	if errors.As(err, &exit) && (exit.ExitCode() == 1 || exit.ExitCode() == 130) {
		return nil
	}
	return err
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
