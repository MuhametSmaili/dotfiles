#!/usr/bin/env bash
set -euo pipefail

# Set HELPERS_DIR if not already set (for standalone execution)
HELPERS_DIR="${HELPERS_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"

source "$HELPERS_DIR/utils.sh"

setup_ssh() {
	echo "🔐 Setting up SSH config..."
	if ask_yes_no "Import work SSH config from 1Password?"; then
		sync_ssh_config pull work
	else
		skip_with_message "Skipping work SSH config import."
	fi
}

ssh_usage() {
	cat <<'EOF'
Usage: bash install/helpers/ssh.sh [action]

  (no arguments)                     Import work config (existing installer behavior)
  pull personal [vault]              Restore SSH_PERSONAL_CONFIG to ~/.ssh/config.local
  push personal [vault]              Create/update SSH_PERSONAL_CONFIG from config.local
  pull work [vault]                  Restore SSH_WORK_CONFIG to ~/.ssh/config.work
  push work <vault>                  Create/update SSH_WORK_CONFIG from config.work
Personal actions default to the Private vault. Work pulls search
accessible vaults unless a vault is specified; work pushes require a vault.
Existing files/backups require confirmation. DRY_RUN=true makes no changes and
does not contact 1Password. Push requires jq. SSH Bookmarks manage key selection
and public-key files; this helper handles configuration Documents only.
EOF
}

require_op() {
	if ! has op; then
		echo "⚠ Install the 1Password CLI before continuing."
		return 1
	fi
}

# Download beside the destination, validate, then replace only after approval.
restore_ssh_config() (
	local destination="$1"
	shift
	if [[ -L "$destination" || (-e "$destination" && ! -f "$destination") ]]; then
		echo "⚠ Refusing to replace a symlink or non-regular file: $destination"
		return 1
	fi
	if ${DRY_RUN:-false}; then
		echo "[DRY RUN] Would download and confirm before replacing $destination"
		return 0
	fi
	require_op || return 1
	umask 077
	mkdir -p -m 700 "$HOME/.ssh" || return 1
	local tmp
	tmp=$(mktemp "${destination}.XXXXXX") || return 1
	trap 'rm -f -- "$tmp"' EXIT
	trap 'exit 1' HUP INT TERM
	if ! "$@" >"$tmp" || [[ ! -s "$tmp" ]]; then
		echo "⚠ Download failed or was empty; existing file preserved."
		return 1
	fi
	if grep -Eq -- '-----BEGIN .*PRIVATE KEY-----' "$tmp"; then
		echo "⚠ Refusing configuration containing private-key material."
		return 1
	fi
	if [[ -f "$destination" ]]; then
		if cmp -s "$tmp" "$destination"; then
			echo "✓ Already up to date: $destination"
			return 0
		fi
		if ! ask_yes_no "Replace $destination with the 1Password copy?"; then
			skip_with_message "Existing file preserved."
			return 0
		fi
	fi
	# Recheck after download/confirmation in case the destination changed.
	if [[ -L "$destination" || (-e "$destination" && ! -f "$destination") ]] || ! mv -f -- "$tmp" "$destination"; then
		echo "⚠ Could not restore file; existing destination preserved."
		return 1
	fi
	echo "✓ Restored $destination"
)

sync_ssh_config() (
	local action="$1" profile="$2" vault="${3:-}" document destination
	case "$profile" in
	personal)
		document=SSH_PERSONAL_CONFIG
		destination="$HOME/.ssh/config.local"
		vault="${vault:-Private}"
		;;
	work)
		document=SSH_WORK_CONFIG
		destination="$HOME/.ssh/config.work"
		;;
	*)
		ssh_usage
		return 1
		;;
	esac
	local -a args=(document get "$document")
	[[ -z "$vault" ]] || args+=(--vault "$vault")
	if [[ "$action" == pull ]]; then
		restore_ssh_config "$destination" op "${args[@]}"
		return $?
	fi
	if [[ -z "$vault" ]]; then
		echo "⚠ Specify the destination vault for a work-config push."
		return 1
	fi
	if [[ ! -f "$destination" || ! -s "$destination" || -L "$destination" ]]; then
		echo "⚠ Expected a nonempty regular config file: $destination"
		return 1
	fi
	if grep -Eq -- '-----BEGIN .*PRIVATE KEY-----' "$destination"; then
		echo "⚠ Refusing to upload configuration containing private-key material."
		return 1
	fi
	if ${DRY_RUN:-false}; then
		echo "[DRY RUN] Would confirm and create/update $document in vault $vault from $destination"
		return 0
	fi
	require_op || return 1
	if ! has jq; then
		echo "⚠ Install jq before pushing a config."
		return 1
	fi
	local item_id
	# A failed lookup must never be interpreted as a missing document.
	if ! item_id=$(op document list --vault "$vault" --format json | jq -rs --arg title "$document" '
		if length != 1 or (.[0] | type) != "array" then error("Invalid document listing") else .[0] end |
		[.[] | select(.title == $title)] |
		if length == 0 then ""
		elif length == 1 then
			.[0].id | if type == "string" and length > 0 then . else error("Invalid document ID") end
		else error("Multiple matching documents; resolve duplicates in 1Password first") end
	'); then
		echo "⚠ Could not identify the backup safely; nothing uploaded."
		return 1
	fi
	if [[ -n "$item_id" ]]; then
		if ! ask_yes_no "Update $document in vault $vault from $destination?"; then
			skip_with_message "Backup unchanged."
			return 0
		fi
		op document edit "$item_id" "$destination" --vault "$vault" >/dev/null || return 1
	else
		if ! ask_yes_no "Create $document in vault $vault from $destination?"; then
			skip_with_message "No backup created."
			return 0
		fi
		op document create "$destination" --title "$document" --vault "$vault" >/dev/null || return 1
	fi
	echo "✓ Backed up $document to vault $vault"
)

ssh_main() {
	case "${1:-}" in
	"") setup_ssh ;;
	-h | --help) ssh_usage ;;
	pull | push)
		if [[ $# -lt 2 || $# -gt 3 ]]; then
			ssh_usage
			return 1
		fi
		sync_ssh_config "$@"
		;;
	*)
		ssh_usage
		return 1
		;;
	esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
	ssh_main "$@"
else
	# Preserve the installer's sourced, work-only import regardless of its arguments.
	setup_ssh
fi
