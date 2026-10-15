# Dotfiles

These are my personal dotfiles for a development setup.

This is a personal configuration. If you do not like a part of it, change that part or select only the parts that you want.

<img width="2874" height="1620" alt="Neovim setup screenshot" src="https://github.com/user-attachments/assets/2c110a4b-cfba-4273-885f-c1ecaaf8d396" />

## Quick Setup

Read the script before you run it.

```bash
git clone https://github.com/MuhametSmaili/dotfiles ~/dotfiles
cd ~/dotfiles
./install.sh
```

To see the planned actions before you make changes, use the dry-run mode:

```bash
./install.sh --dry-run
```

The dry-run mode prints each command. It does not change the system.

## Supported Platforms

| Platform | Package manager |
| -------- | --------------- |
| macOS    | Homebrew        |
| Ubuntu   | APT             |
| Fedora   | DNF             |

The install script detects the platform. Then it runs the correct package manager.

## What the Setup Includes

### Applications

- **Terminal**: Ghostty
- **Window manager (macOS)**: yabai with skhd for the key bindings
- **Launcher (macOS)**: Vicinae
- **Tmux sessions**: hetki
- **Development**: Neovim, Lazygit, Lazydocker, Yazi, btop, GitHub CLI
- **Coding agents**: Kiro, Pi, opencode

### Shell

- **Shell**: Zsh
- **Plugin manager**: Zinit
- **Prompt**: Pure
- **Plugins**: autosuggestions, fast-syntax-highlighting, completions, fzf-tab
- **Tools**: fzf, fd, bat, ripgrep, delta, tmux

### Configurations

| Path                | Purpose                        |
| ------------------- | ------------------------------ |
| `.zshrc`            | Shell configuration            |
| `.config/nvim/`     | Neovim setup                   |
| `.config/tmux/`     | Tmux configuration             |
| `.config/yazi/`     | Yazi file manager              |
| `.config/git/`      | Git configuration and delta    |
| `.config/gh/`       | GitHub CLI preferences         |
| `.config/fzp/`      | Channels for the `fzp` picker  |
| `.config/scripts/`  | Personal scripts on the `PATH` |
| `.config/opencode/` | opencode configuration         |
| `.kiro/`            | Kiro agents and settings       |
| `.pi/agent/`        | Pi agent configuration         |

The repository also holds configurations for Aerospace and WezTerm. The install script does not install these two tools. The configurations stay in the repository for other machines.

## How the Linking Works

GNU Stow makes the symbolic links from the repository into `$HOME`. Stow does not copy the files. Each linked path points to one file in the repository. If you edit the file through either path, you edit the same file.

Run this command after you add a file to the repository or remove a file from it:

```bash
restow
```

The `restow` alias comes from `.zshrc`. Existing links continue to work without this command. Only a new file or a removed file needs it.

## Manual Steps After the Install

### 1. Install the tmux plugins

Start tmux. Then press:

```text
prefix + I
```

The prefix is `Ctrl-a`.

### 2. Install hetki

The Brewfile and the mise configuration do not provide hetki. Install it with Go:

```bash
go install github.com/MuhametSmaili/hetki@latest
```

hetki starts and switches the tmux sessions. The `prefix + o` key binding opens
it. The workspace files are in `~/.config/hetki/workspaces/`. Each file names
the sessions, the windows, and the paths. This repository does not hold these
files, because the sessions differ on each machine.

To update the tool later, run:

```bash
hetki update
```

### 3. Recommended versions

Use at least these versions for full compatibility with the scripts here:

- tmux 3.2a or later
- bash 5.x or later

## Local Configuration

Git ignores the files below. Use them for personal settings.

`.zshrc.local` holds personal shell settings. `.zshrc` reads this file.

```bash
# Example: personal paths, API keys, or work aliases
export WORK_DIR="$HOME/work"
```

`.config/git/config.local` holds the personal Git identity. `.config/git/config` includes this file.

```bash
[user]
    name = Your Name
    email = your.email@example.com
    signingkey = YOUR_GPG_KEY
```
