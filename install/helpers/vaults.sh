#!/usr/bin/env bash
set -euo pipefail

HELPERS_DIR="${HELPERS_DIR:-$(cd "$(dirname "$0")" && pwd)}"
source "$HELPERS_DIR/utils.sh"

VAULTS_REPO_DIR="${VAULTS_REPO_DIR:-$HOME/.vaults}"
VAULTS_REPO_URL="${VAULTS_REPO_URL:-git@github.com:MuhametSmaili/.vaults.git}"

ensure_vault_repo() {
    echo "📚 Setting up vault repository..."

    if [[ ! -d "$VAULTS_REPO_DIR/.git" ]]; then
        run_cmd git clone "$VAULTS_REPO_URL" "$VAULTS_REPO_DIR"
    fi
}

setup_vault_hooks_and_pull() {
    echo "🧩 Configuring vault hooks and decrypting notes..."
    run_cmd make -C "$VAULTS_REPO_DIR" hooks
    run_cmd make -C "$VAULTS_REPO_DIR" pull
}

main() {
    if ! ask_yes_no "Setup/update encrypted vault at $VAULTS_REPO_DIR?" "Y"; then
        skip_with_message "Skipping vault setup."
        return 0
    fi

    ensure_vault_repo
    setup_vault_hooks_and_pull

    echo "✅ Vault setup complete"
}

main
