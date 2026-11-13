// Package catalog finds channel definitions in the channels directory.
//
// A channel is <name>.yaml, or a bundle <name>/main.yaml that keeps its
// scripts next to it; the bundle wins when both exist. A channel with steps:
// or run: is a flow; any other channel is a single fzf screen.
package catalog

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Kind is how a channel runs.
type Kind int

const (
	// Screen is one fzf list with key bindings.
	Screen Kind = iota
	// Flow is a sequence of selection steps followed by one command.
	Flow
)

// TargetRequired as metadata.target means the channel acts on a tmux pane
// and must not start without one.
const TargetRequired = "required"

// Metadata is the metadata: block shared by both kinds of channel.
type Metadata struct {
	Description string   `yaml:"description"`
	Group       string   `yaml:"group"`
	Requires    []string `yaml:"requires"`
	Target      string   `yaml:"target"`
}

// Channel is one channel definition.
type Channel struct {
	Name     string
	Path     string
	Kind     Kind
	Metadata Metadata
}

// Dir is the directory holding the channel's YAML file.
func (c Channel) Dir() string {
	return filepath.Dir(c.Path)
}

// header is the part of a channel file the catalog reads; each kind decodes
// the full file strictly when it runs.
type header struct {
	Metadata Metadata  `yaml:"metadata"`
	Steps    yaml.Node `yaml:"steps"`
	Run      yaml.Node `yaml:"run"`
}

// List returns every channel in dir, sorted by name.
func List(dir string) ([]Channel, error) {
	paths, err := channelPaths(dir)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)

	channels := make([]Channel, 0, len(names))
	for _, name := range names {
		channel, err := read(name, paths[name])
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, nil
}

// Find returns the channel called name.
func Find(dir, name string) (Channel, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return Channel{}, fmt.Errorf("invalid channel name %q", name)
	}

	for _, path := range []string{filepath.Join(dir, name, "main.yaml"), filepath.Join(dir, name+".yaml")} {
		if isFile(path) {
			return read(name, path)
		}
	}
	return Channel{}, fmt.Errorf("channel not found: %s", name)
}

func channelPaths(dir string) (map[string]string, error) {
	if stat, err := os.Stat(dir); err != nil || !stat.IsDir() {
		return nil, fmt.Errorf("channels directory not found: %s", dir)
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}

	bundles, err := filepath.Glob(filepath.Join(dir, "*", "main.yaml"))
	if err != nil {
		return nil, err
	}

	paths := map[string]string{}
	for _, path := range files {
		paths[strings.TrimSuffix(filepath.Base(path), ".yaml")] = path
	}
	for _, path := range bundles {
		paths[filepath.Base(filepath.Dir(path))] = path
	}
	return paths, nil
}

func read(name, path string) (Channel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Channel{}, err
	}

	var h header
	if err := yaml.Unmarshal(data, &h); err != nil {
		return Channel{}, fmt.Errorf("%s: %w", path, err)
	}

	if h.Metadata.Target != "" && h.Metadata.Target != TargetRequired {
		return Channel{}, fmt.Errorf("%s: metadata.target must be %q or omitted", path, TargetRequired)
	}

	kind := Screen
	if h.Steps.Kind != 0 || h.Run.Kind != 0 {
		kind = Flow
	}
	return Channel{Name: name, Path: path, Kind: kind, Metadata: h.Metadata}, nil
}

// MissingTools reports the required tools that are not on PATH.
func (m Metadata) MissingTools() error {
	var missing []string
	for _, tool := range m.Requires {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		return errors.New("missing dependencies: " + strings.Join(missing, " "))
	}
	return nil
}

func isFile(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && !stat.IsDir()
}
