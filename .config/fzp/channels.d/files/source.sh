#!/usr/bin/env bash
# Lists files as "<icon>\t<path>". FZP_FILES_INCLUDE_IGNORED=1 includes
# gitignored files.
set -euo pipefail

args=(--type f --hidden --follow --exclude .git)
if [[ "${FZP_FILES_INCLUDE_IGNORED:-0}" == 1 ]]; then
	args+=(--no-ignore)
fi

if command -v devicon-lookup >/dev/null 2>&1; then
	fd "${args[@]}" | devicon-lookup | awk '{ icon = $1; $1 = ""; sub(/^ /, ""); print icon "\t" $0 }'
else
	fd "${args[@]}" | awk '{ print "  \t" $0 }'
fi
