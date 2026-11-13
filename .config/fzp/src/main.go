// Command fzp is a channel-based launcher built on fzf.
//
// A channel is a YAML file in $XDG_CONFIG_HOME/fzp/channels.d. A screen
// channel is one fzf list with key bindings; a flow channel is a sequence of
// selection steps followed by one command.
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

  fzp                  pick a channel, then run it
  fzp files            run the files channel
  fzp --group pane     pick among channels whose metadata.group is "pane"
  fzp --list           list channel names
  fzp --check          validate every channel

Flags:
  --query TEXT         initial query
  --target PANE        tmux pane to act on, e.g. #{pane_id} from a binding;
                       exported as FZP_PANE_ID, FZP_PANE_CWD, FZP_SESSION_ID, ...
  --client CLIENT      tmux client that opened fzp, exported as FZP_CLIENT
  --group GROUP        limit the channel menu to one metadata.group
  --scope FILE         restrict file channels to the paths listed in FILE,
                       exported as FZP_SCOPE_FILE

Environment:
  FZP_HEIGHT           picker height (default 100%%)
  XDG_CONFIG_HOME      channels live in $XDG_CONFIG_HOME/fzp/channels.d

Channels dir: %s
`

type options struct {
	query  string
	target string
	client string
	group  string
	scope  string
	list   bool
	check  bool
	args   []string
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := newApp(paths, opts)
	if err == nil {
		err = app.run(ctx, opts)
	}

	if err != nil {
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
	flags.StringVar(&opts.target, "target", os.Getenv("FZP_TARGET"), "")
	flags.StringVar(&opts.client, "client", os.Getenv("FZP_CLIENT"), "")
	flags.StringVar(&opts.group, "group", "", "")
	flags.StringVar(&opts.scope, "scope", os.Getenv("FZP_SCOPE_FILE"), "")
	flags.BoolVar(&opts.list, "list", false, "")
	flags.BoolVar(&opts.check, "check", false, "")

	if err := flags.Parse(args); err != nil {
		return options{}, err
	}

	opts.args = flags.Args()
	if len(opts.args) > 1 {
		_, _ = fmt.Fprintf(stderr, "fzp: expected at most one channel, got %q\n", opts.args)
		return options{}, errors.New("too many arguments")
	}
	return opts, nil
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
