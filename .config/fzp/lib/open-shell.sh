#!/usr/bin/env bash
# Opens a shell in DIR next to the target pane, as chosen by a "place" step
# whose items carry kind (window, horizontal, vertical) and focus fields.
#
# Usage: open-shell.sh DIR [WINDOW_NAME]
set -euo pipefail

dir="$1"
name="${2:-$(basename "$dir")}"

case "$FZP_STEP_PLACE_KIND" in
	window)
		pane=$(tmux new-window -d -P -F '#{pane_id}' -t "$FZP_SESSION_ID:" -c "$dir" -n "$name")
		;;
	horizontal | vertical)
		flag=-v
		[[ "$FZP_STEP_PLACE_KIND" == horizontal ]] && flag=-h
		pane=$(tmux split-window -d -P -F '#{pane_id}' "$flag" -t "$FZP_PANE_ID" -c "$dir")
		;;
	*)
		echo "unknown placement: $FZP_STEP_PLACE_KIND" >&2
		exit 1
		;;
esac

if [[ "$FZP_STEP_PLACE_FOCUS" == true ]]; then
	tmux select-window -t "$pane" \; select-pane -t "$pane"
fi
