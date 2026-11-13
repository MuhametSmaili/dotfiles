#!/usr/bin/env bash
# Searches for $1 in the project, or only in the files listed in
# FZP_SCOPE_FILE when grep was opened from files with a query.
set -uo pipefail

query="${1:-}"
[[ -n "$query" ]] || exit 0

args=(--color=always --line-number --no-heading --smart-case --field-match-separator=$'\t' -- "$query")

if [[ -n "${FZP_SCOPE_FILE:-}" ]]; then
	# An empty scope means the files query matched nothing.
	[[ -s "$FZP_SCOPE_FILE" ]] || exit 0
	tr '\n' '\0' <"$FZP_SCOPE_FILE" | xargs -0 rg "${args[@]}"
else
	rg "${args[@]}" .
fi

# rg exits 1 when nothing matches; that is an empty list, not an error.
exit 0
