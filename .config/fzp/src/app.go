package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fzp/internal/catalog"
	"fzp/internal/flow"
	"fzp/internal/fzf"
	"fzp/internal/screen"
	"fzp/internal/tmux"
)

// app runs channels. Every command a channel starts inherits environ, so a
// channel reached through a navigation key keeps the target and scope.
type app struct {
	paths   paths
	self    string
	height  string
	environ []string
}

func newApp(p paths, opts options) (*app, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate fzp: %w", err)
	}

	environ := append(os.Environ(), "FZP_CONFIG_DIR="+p.config)
	if opts.scope != "" {
		scope, err := filepath.Abs(opts.scope)
		if err != nil {
			return nil, err
		}
		environ = append(environ, "FZP_SCOPE_FILE="+scope)
	}

	a := &app{
		paths:   p,
		self:    self,
		height:  firstNonEmpty(os.Getenv("FZP_HEIGHT"), "100%"),
		environ: environ,
	}
	return a, nil
}

func (a *app) run(ctx context.Context, opts options) error {
	switch {
	case opts.list:
		return a.printList()
	case opts.check:
		return a.check()
	case len(opts.args) == 1:
		err := a.runChannel(ctx, opts.args[0], opts)
		return quiet(err)
	default:
		return a.menu(ctx, opts)
	}
}

// quiet turns leaving a channel into success.
func quiet(err error) error {
	if errors.Is(err, flow.ErrBackedOut) || errors.Is(err, flow.ErrQuit) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// menu lets the user pick a channel and shows the menu again when a flow is
// left with Esc on its first step.
func (a *app) menu(ctx context.Context, opts options) error {
	for {
		name, err := a.pickChannel(ctx, opts)
		if err != nil || name == "" {
			return err
		}

		err = a.runChannel(ctx, name, opts)
		if !errors.Is(err, flow.ErrBackedOut) {
			return quiet(err)
		}

		opts.query = ""
	}
}

func (a *app) pickChannel(ctx context.Context, opts options) (string, error) {
	channels, err := catalog.List(a.paths.channels)
	if err != nil {
		return "", err
	}

	var lines strings.Builder
	for _, c := range channels {
		if opts.group == "" || c.Metadata.Group == opts.group {
			fmt.Fprintf(&lines, "%s\t%s\t%s\n", c.Name, fzf.Sanitize(c.Metadata.Description, false), c.Path)
		}
	}

	label := firstNonEmpty(opts.group, "channel")
	result, err := fzf.Pick(ctx, fzf.Prompt{
		Args: []string{
			"--height", a.height,
			"--prompt", label + "> ",
			"--header", "pick a " + label,
			"--query", opts.query,
			"--delimiter", "\t",
			"--with-nth", "1,2",
			"--with-shell", "bash -c",
			"--preview", "bat --color=always --style=plain {3} 2>/dev/null || cat {3}",
		},
		Input: lines.String(),
		Env:   a.environ,
	})
	if err != nil || result.Aborted {
		return "", err
	}

	name, _, _ := strings.Cut(result.Line, "\t")
	return name, nil
}

func (a *app) runChannel(ctx context.Context, name string, opts options) error {
	channel, err := catalog.Find(a.paths.channels, name)
	if err != nil {
		return err
	}

	if err := channel.Metadata.MissingTools(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	target, err := a.resolveTarget(ctx, channel, opts)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	environ := append(append([]string{}, a.environ...), target.Environ()...)
	environ = append(environ, "FZP_TARGET="+target.PaneID, "FZP_CHANNEL_DIR="+channel.Dir())

	if channel.Kind == catalog.Flow {
		return a.runFlow(ctx, channel, target, environ)
	}
	return a.runScreen(ctx, channel, opts, environ)
}

// resolveTarget describes the pane the channel acts on. A channel that
// requires a target falls back to the pane fzp runs in.
func (a *app) resolveTarget(ctx context.Context, channel catalog.Channel, opts options) (tmux.Target, error) {
	pane := opts.target
	required := channel.Metadata.Target == catalog.TargetRequired
	if pane == "" && required {
		pane = os.Getenv("TMUX_PANE")
	}

	if pane == "" && required {
		return tmux.Target{}, errors.New("this channel acts on a tmux pane: run it inside tmux or pass --target")
	}
	return tmux.Resolve(ctx, pane, opts.client)
}

func (a *app) runFlow(ctx context.Context, channel catalog.Channel, target tmux.Target, environ []string) error {
	def, err := flow.Load(channel)
	if err != nil {
		return err
	}

	return def.Run(ctx, flow.Options{
		Channel: channel.Name,
		Height:  a.height,
		Target:  target,
		Environ: environ,
	})
}

func (a *app) runScreen(ctx context.Context, channel catalog.Channel, opts options, environ []string) error {
	spec, err := screen.Load(channel)
	if err != nil {
		return err
	}

	return spec.Run(ctx, screen.Options{
		Channel: channel.Name,
		Dir:     channel.Dir(),
		Query:   opts.query,
		Height:  a.height,
		Self:    a.self,
		Env:     environ,
	})
}

func (a *app) printList() error {
	channels, err := catalog.List(a.paths.channels)
	if err != nil {
		return err
	}

	for _, c := range channels {
		fmt.Println(c.Name)
	}
	return nil
}

// check loads every channel and reports all invalid ones.
func (a *app) check() error {
	channels, err := catalog.List(a.paths.channels)
	if err != nil {
		return err
	}

	var failures []error
	for _, c := range channels {
		if err := load(c); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", c.Name, err))
		}
	}

	if err := errors.Join(failures...); err != nil {
		return err
	}

	fmt.Printf("%d channels are valid\n", len(channels))
	return nil
}

func load(c catalog.Channel) error {
	if c.Kind == catalog.Flow {
		_, err := flow.Load(c)
		return err
	}

	_, err := screen.Load(c)
	return err
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
