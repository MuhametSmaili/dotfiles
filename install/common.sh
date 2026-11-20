#!/usr/bin/env bash
set -euo pipefail

source "$HELPERS_DIR/utils.sh"
source "$HELPERS_DIR/pi-agent.sh"

# Stow links a folder as a whole when it is not in $HOME yet.
# Apps write state to these folders, so make them first. Then stow links each file.
PER_FILE_DIRS=(
	.config/zsh
	.config/tmux # TPM plugins
	.config/hetki # workspaces/
	.config/gh # hosts.yml has the GitHub login
	.ssh # known_hosts, config.work, 1Password/, agent/
	.kiro/agents
	.kiro/settings
	.pi/agent/extensions
	.pi/agent/themes
)
run_cmd mkdir -p "${PER_FILE_DIRS[@]/#/$HOME/}"

# Apps write to these files, so a link does not work. Copy them one time.
# Each item is "file in dotfiles:file in $HOME".
COPY_ONCE=(
	.kiro/settings/cli.json:.kiro/settings/cli.json
	.config/vicinae/shortcuts.json:.local/share/vicinae/shortcuts/shortcuts.json
)
for item in "${COPY_ONCE[@]}"; do
	src="$DOTFILES_DIR/${item%%:*}" dst="$HOME/${item#*:}"
	[[ -e "$dst" ]] || { run_cmd mkdir -p "$(dirname "$dst")" && run_cmd cp "$src" "$dst"; }
done

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

echo "✅ Common setup complete!"
