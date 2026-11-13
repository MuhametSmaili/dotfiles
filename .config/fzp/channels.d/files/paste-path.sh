#!/usr/bin/env bash
# Types $1 into the target pane (fzp --target). Without a usable target, the
# path goes to the tmux buffer inside tmux, or the clipboard outside it.
set -euo pipefail

path="${1:-}"
[[ -n "$path" ]] || exit 0

if [[ -n "${TMUX:-}" ]]; then
	if [[ -n "${FZP_PANE_ID:-}" ]] && tmux send-keys -t "$FZP_PANE_ID" -l -- "$path" 2>/dev/null; then
		exit 0
	fi

	tmux set-buffer -- "$path"
	tmux display-message "fzp: no target pane, copied to the tmux buffer"
	exit 0
fi

printf '%s' "$path" | pbcopy
