// Command fzp is a channel-based launcher built on fzf.
//
// A channel is a YAML file in $XDG_CONFIG_HOME/fzp/channels.d: one or more
// fzf steps whose rows come from shell commands, optionally followed by a
// command that receives the picks. See package channel for the format.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

const usage = `Usage: fzp [flags] [channel]

  fzp                    pick a category, then a channel
  fzp files              run the files channel
  fzp --group Git        open the Git category directly
  fzp --list             list channel names
  fzp --check            validate every channel
  fzp --filter Q files   print the rows of files' first step that match Q

Flags:
  --query TEXT           initial query
  --group GROUP          open one metadata.group (case-insensitive)
  --input-query TEXT     query handed over by another channel, e.g. a key
                         become:fzp --input-query {q} grep; exported as
                         FZP_INPUT_QUERY, and the channel's scope command runs

Environment:
  FZP_HEIGHT             picker height (default 100%%)
  XDG_CONFIG_HOME        channels live in $XDG_CONFIG_HOME/fzp/channels.d

Channels read context from the environment; a tmux binding passes the pane
with, e.g., display-popup -e FZP_PANE_ID=#{pane_id}.

Channels dir: %s
`

type options struct {
	query string
	group string
	list  bool
	check bool
	args  []string
	// inputQuery and filter are nil when the flag is absent; an empty
	// query is a valid value for both.
	inputQuery *string
	filter     *string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	paths := defaultPaths()

	opts, err := parseFlags(args, stderr, paths)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}

	if err != nil {
		return 2
	}

	// Ctrl+C belongs to whatever has the terminal: fzf, or a program a key
	// started. fzp only uses it to stop its own commands (see proc), so it
	// must not die of it. signal.Notify never blocks on the full channel,
	// and unlike signal.Ignore it does not make children ignore SIGINT.
	signal.Notify(make(chan os.Signal, 1), os.Interrupt)

	// A closed popup (SIGHUP) or a kill (SIGTERM) stops fzf and commands.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	if err := newApp(paths, opts).run(ctx, opts); err != nil {
		_, _ = fmt.Fprintln(stderr, "fzp:", err)
		return 1
	}
	return 0
}

func parseFlags(args []string, stderr io.Writer, paths paths) (options, error) {
	var opts options

	flags := flag.NewFlagSet("fzp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { _, _ = fmt.Fprintf(stderr, usage, paths.channels) }

	flags.StringVar(&opts.query, "query", "", "")
	flags.StringVar(&opts.group, "group", "", "")
	flags.BoolVar(&opts.list, "list", false, "")
	flags.BoolVar(&opts.check, "check", false, "")
	inputQuery := flags.String("input-query", "", "")
	filter := flags.String("filter", "", "")

	if err := flags.Parse(args); err != nil {
		return options{}, err
	}

	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "input-query":
			opts.inputQuery = inputQuery
		case "filter":
			opts.filter = filter
		}
	})

	opts.args = flags.Args()
	if err := checkArgs(opts); err != nil {
		_, _ = fmt.Fprintln(stderr, "fzp:", err)
		return options{}, err
	}
	return opts, nil
}

func checkArgs(opts options) error {
	switch {
	case len(opts.args) > 1:
		return fmt.Errorf("expected at most one channel, got %q", opts.args)
	case opts.filter != nil && len(opts.args) != 1:
		return errors.New("--filter needs a channel")
	default:
		return nil
	}
}

// paths are where fzp finds its configuration.
type paths struct {
	config   string
	channels string
}

func defaultPaths() paths {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}

	config := filepath.Join(base, "fzp")
	return paths{config: config, channels: filepath.Join(config, "channels.d")}
}
