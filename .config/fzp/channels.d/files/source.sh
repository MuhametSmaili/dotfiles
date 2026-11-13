#!/usr/bin/env bash
# Lists files as "<icon>\t<path>". With FZP_SCOPE_FILE, lists only the paths
# in that file; FZP_FILES_INCLUDE_IGNORED=1 includes gitignored files.
set -euo pipefail

list() {
	if [[ -n "${FZP_SCOPE_FILE:-}" ]]; then
		sort -u "$FZP_SCOPE_FILE"
		return
	fi

	local -a args=(--type f --hidden --follow --exclude .git)
	if [[ "${FZP_FILES_INCLUDE_IGNORED:-0}" == 1 ]]; then
		args+=(--no-ignore)
	fi
	fd "${args[@]}"
}

if command -v devicon-lookup >/dev/null 2>&1; then
	list | devicon-lookup | awk '{ icon = $1; $1 = ""; sub(/^ /, ""); print icon "\t" $0 }'
else
	list | awk '{ print "  \t" $0 }'
fi
