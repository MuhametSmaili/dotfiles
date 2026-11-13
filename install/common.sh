#!/usr/bin/env bash
set -euo pipefail

source "$HELPERS_DIR/utils.sh"
source "$HELPERS_DIR/pi-agent.sh"
source "$HELPERS_DIR/kiro.sh"

run_cmd mkdir -p ~/.config/zsh ~/.config/tmux

install_zsh
install_tmux_plugins

if ask_yes_no "Install/update mise?"; then
	install_mise
else
	skip_with_message "Skipping Mise installation."
fi

if ask_yes_no "Install/update Pi?"; then
	install_pi_agent
else
	skip_with_message "Skipping Pi installation."
fi

if ask_yes_no "Install/update SonarLint?"; then
	source "$HELPERS_DIR/sonarlint.sh"
	install_sonarlint
else
	skip_with_message "Skipping SonarLint installation."
fi

if has stow; then
	echo "🔗 Linking dotfiles..."
	prepare_pi_agent
	prepare_kiro
	run_cmd stow \
		--dir="$DOTFILES_DIR" \
		--target="$HOME" \
		--restow \
		.
else
	echo "⚠️ stow not installed, skipping linking."
fi

if has ya && [[ -f "$HOME/.config/yazi/package.toml" ]]; then
	echo "📦 Installing Yazi packages..."
	run_cmd ya pkg install
fi

# Register Vicinae script commands (idempotent; skips if Vicinae absent).
source "$HELPERS_DIR/vicinae.sh"
link_vicinae_scripts

echo "✅ Common setup complete!"
