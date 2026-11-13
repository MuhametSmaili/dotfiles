#!/usr/bin/env bash
# Copies stdin to the clipboard on macOS and Linux (Wayland or X11). Without
# a local clipboard, as over SSH, it uses tmux's buffer, which tmux forwards to
# the terminal, or asks the terminal directly with OSC 52.
set -euo pipefail

if command -v pbcopy >/dev/null 2>&1; then
	exec pbcopy
fi

if [[ -n "${WAYLAND_DISPLAY:-}" ]] && command -v wl-copy >/dev/null 2>&1; then
	exec wl-copy
fi

if [[ -n "${DISPLAY:-}" ]] && command -v xclip >/dev/null 2>&1; then
	exec xclip -selection clipboard
fi

if [[ -n "${TMUX:-}" ]]; then
	exec tmux load-buffer -w -
fi

printf '\033]52;c;%s\a' "$(base64 | tr -d '\n')" >/dev/tty
