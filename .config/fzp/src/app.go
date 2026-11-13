package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"fzp/internal/catalog"
	"fzp/internal/channel"
)

// app runs channels. Every command a channel starts inherits environ, so
// context the caller passed in, such as FZP_PANE_ID from a tmux binding,
// reaches every channel opened from there.
type app struct {
	paths   paths
	height  string
	environ []string
}

func newApp(p paths, opts options) *app {
	environ := append(withoutInternal(os.Environ()), "FZP_CONFIG_DIR="+p.config)
	if opts.inputQuery != nil {
		environ = append(environ, "FZP_INPUT_QUERY="+*opts.inputQuery)
	}

	return &app{
		paths:   p,
		height:  cmp.Or(os.Getenv("FZP_HEIGHT"), "100%"),
		environ: environ,
	}
}

func (a *app) run(ctx context.Context, opts options) error {
	switch {
	case opts.list:
		return a.printList()
	case opts.check:
		return a.check(ctx)
	case opts.filter != nil:
		return a.filter(ctx, opts.args[0], *opts.filter)
	case len(opts.args) == 1:
		return quiet(a.runChannel(ctx, opts.args[0], opts))
	default:
		return a.menu(ctx, opts)
	}
}

// quiet turns leaving a channel into success.
func quiet(err error) error {
	if errors.Is(err, channel.ErrBackedOut) || errors.Is(err, channel.ErrQuit) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// menu lets the user pick a channel, and shows the menu again when a channel
// is left with Esc on its first step.
func (a *app) menu(ctx context.Context, opts options) error {
	for {
		name, err := a.pickChannel(ctx, opts)
		if err != nil || name == "" {
			return quiet(err)
		}

		err = a.runChannel(ctx, name, options{inputQuery: opts.inputQuery})
		if !errors.Is(err, channel.ErrBackedOut) {
			return quiet(err)
		}
	}
}

// pickChannel shows the channel menu as a one-step channel. Channels whose
// YAML is unreadable are left out; fzp --check reports them.
func (a *app) pickChannel(ctx context.Context, opts options) (string, error) {
	channels, _ := catalog.List(a.paths.channels)

	var items []map[string]string
	for _, c := range channels {
		if opts.group == "" || c.Metadata.Group == opts.group {
			description := strings.NewReplacer("\t", " ", "\n", " ").Replace(c.Metadata.Description)
			items = append(items, map[string]string{"name": c.Name, "description": description, "path": c.Path})
		}
	}

	if len(items) == 0 {
		return "", fmt.Errorf("no channels in %s", a.paths.channels)
	}

	label := cmp.Or(opts.group, "channel")
	return channel.Pick(ctx, channel.Options{Name: label, Height: a.height, Query: opts.query, Env: a.environ}, channel.Step{
		Title:   "pick a " + label,
		Items:   items,
		Columns: []string{"name", "description", "path"},
		Show:    []string{"name", "description"},
		Preview: "bat --color=always --style=plain {3} 2>/dev/null || cat {3}",
	})
}

func (a *app) runChannel(ctx context.Context, name string, opts options) error {
	found, spec, environ, err := a.load(name)
	if err != nil {
		return err
	}

	if err := found.Metadata.Check(environ); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	return spec.Run(ctx, channel.Options{
		Name:     name,
		Height:   a.height,
		Query:    opts.query,
		Env:      environ,
		HasInput: opts.inputQuery != nil,
	})
}

// filter prints the rows of a channel's first step that match query.
func (a *app) filter(ctx context.Context, name, query string) error {
	_, spec, environ, err := a.load(name)
	if err != nil {
		return err
	}
	return spec.Filter(ctx, channel.Options{Name: name, Env: environ}, query, os.Stdout)
}

func (a *app) load(name string) (catalog.Channel, channel.Spec, []string, error) {
	found, err := catalog.Find(a.paths.channels, name)
	if err != nil {
		return catalog.Channel{}, channel.Spec{}, nil, err
	}

	spec, err := channel.Load(found.Path)
	if err != nil {
		return catalog.Channel{}, channel.Spec{}, nil, err
	}

	environ := append(append([]string{}, a.environ...), "FZP_CHANNEL_DIR="+found.Dir())
	return found, spec, environ, nil
}

func (a *app) printList() error {
	channels, err := catalog.List(a.paths.channels)
	for _, c := range channels {
		fmt.Println(c.Name)
	}
	return err
}

// check verifies fzf, loads every channel and reports every problem.
func (a *app) check(ctx context.Context) error {
	channels, listErr := catalog.List(a.paths.channels)

	failures := []error{channel.CheckFzf(ctx), listErr}
	for _, c := range channels {
		if _, err := channel.Load(c.Path); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", c.Name, err))
		}
	}

	if err := errors.Join(failures...); err != nil {
		return err
	}

	fmt.Printf("fzf is new enough; %d channels are valid\n", len(channels))
	return nil
}

// withoutInternal drops state a parent fzp passed to its own commands, so a
// channel only sees an input query or scope when it was handed one.
func withoutInternal(environ []string) []string {
	internal := []string{"FZP_INPUT_QUERY=", "FZP_SCOPE_FILE=", "FZP_SOURCE=", "FZP_RESULT=", "FZP_CHANNEL_DIR="}

	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		if !hasAnyPrefix(entry, internal) {
			kept = append(kept, entry)
		}
	}
	return kept
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
