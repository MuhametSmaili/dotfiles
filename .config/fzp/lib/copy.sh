#!/usr/bin/env bash
# Copies stdin to the clipboard on macOS and Linux (Wayland or X11). Without
# a local clipboard, or over SSH, it uses tmux's buffer, which tmux forwards to
# the terminal, or asks the terminal directly with OSC 52.
set -euo pipefail

if command -v pbcopy >/dev/null 2>&1; then
	exec pbcopy
fi

if [[ -n "${WAYLAND_DISPLAY:-}" ]] && command -v wl-copy >/dev/null 2>&1; then
	exec wl-copy
fi

# Not over SSH: there DISPLAY is clip-display's virtual display, which holds
# the image clip-upload sent. Copying text into it would replace that image,
# and the text would never reach the local machine anyway.
if [[ -n "${DISPLAY:-}" && -z "${SSH_CONNECTION:-}" ]] && command -v xclip >/dev/null 2>&1; then
	exec xclip -selection clipboard
fi

if [[ -n "${TMUX:-}" ]]; then
	exec tmux load-buffer -w -
fi

printf '\033]52;c;%s\a' "$(base64 | tr -d '\n')" >/dev/tty
