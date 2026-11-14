// Package catalog finds channel definitions in the channels directory.
//
// A channel is <name>.yaml, or a bundle <name>/main.yaml that keeps its
// scripts next to it; the bundle wins when both exist.
package catalog

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Metadata is the metadata: block of a channel.
type Metadata struct {
	Description string `yaml:"description"`
	// Group is the category shown in the root menu; empty stays at the root.
	Group string `yaml:"group"`
	// Requires lists tools that must be on PATH.
	Requires []string `yaml:"requires"`
	// RequiresEnv lists variables the caller must set, such as the pane
	// context a tmux binding passes in.
	RequiresEnv []string `yaml:"requires_env"`
}

// Channel is one channel definition.
type Channel struct {
	Name     string
	Path     string
	Metadata Metadata
}

// Dir is the directory holding the channel's YAML file.
func (c Channel) Dir() string {
	return filepath.Dir(c.Path)
}

// List returns every readable channel in dir, sorted by name, and an error
// describing each channel whose metadata could not be read.
func List(dir string) ([]Channel, error) {
	paths, err := channelPaths(dir)
	if err != nil {
		return nil, err
	}

	var channels []Channel
	var failures []error
	for _, name := range slices.Sorted(maps.Keys(paths)) {
		channel, err := read(name, paths[name])
		if err != nil {
			failures = append(failures, err)
			continue
		}
		channels = append(channels, channel)
	}
	return channels, errors.Join(failures...)
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

// read loads only the metadata; the channel package validates the rest.
func read(name, path string) (Channel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Channel{}, err
	}

	var header struct {
		Metadata Metadata `yaml:"metadata"`
	}
	if err := yaml.Unmarshal(data, &header); err != nil {
		return Channel{}, fmt.Errorf("%s: %w", path, err)
	}
	return Channel{Name: name, Path: path, Metadata: header.Metadata}, nil
}

// Check reports required tools missing from PATH and required variables
// that are unset or empty in environ.
func (m Metadata) Check(environ []string) error {
	var problems []error

	var tools []string
	for _, tool := range m.Requires {
		if _, err := exec.LookPath(tool); err != nil {
			tools = append(tools, tool)
		}
	}
	if len(tools) > 0 {
		problems = append(problems, errors.New("missing dependencies: "+strings.Join(tools, " ")))
	}

	var vars []string
	for _, name := range m.RequiresEnv {
		if lookup(environ, name) == "" {
			vars = append(vars, name)
		}
	}
	if len(vars) > 0 {
		problems = append(problems, errors.New("missing environment: "+strings.Join(vars, " ")))
	}
	return errors.Join(problems...)
}

// lookup returns the last value of name in environ, as exec would use it.
func lookup(environ []string, name string) string {
	value := ""
	for _, entry := range environ {
		if v, ok := strings.CutPrefix(entry, name+"="); ok {
			value = v
		}
	}
	return value
}

func isFile(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && !stat.IsDir()
}
