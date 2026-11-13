#!/usr/bin/env bash
# Types $1 into the pane in FZP_PANE_ID. Without one, the path goes to the
# clipboard.
set -euo pipefail

path="${1:-}"
[[ -n "$path" ]] || exit 0

if [[ -n "${FZP_PANE_ID:-}" ]] && tmux send-keys -t "$FZP_PANE_ID" -l -- "$path" 2>/dev/null; then
	exit 0
fi

printf '%s' "$path" | "$FZP_CONFIG_DIR/lib/copy.sh"
