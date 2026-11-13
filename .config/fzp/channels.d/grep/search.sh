#!/usr/bin/env bash
# Searches for $1 in the project, or only in the files listed in
# FZP_SCOPE_FILE when the files channel narrowed the scope.
set -uo pipefail

query="${1:-}"
[[ -n "$query" ]] || exit 0

args=(--color=always --line-number --no-heading --smart-case -- "$query")

if [[ -n "${FZP_SCOPE_FILE:-}" ]]; then
	tr '\n' '\0' <"$FZP_SCOPE_FILE" | xargs -0 rg "${args[@]}"
else
	rg "${args[@]}" .
fi

# rg exits 1 when nothing matches; that is an empty list, not an error.
exit 0
