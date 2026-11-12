#!/usr/bin/env bash
set -euo pipefail

echo "🐧 Ubuntu setup..."

run_cmd sudo apt update -qq
run_cmd sudo apt install -y git curl stow zsh unzip xclip xvfb xauth

bash "$INSTALL_DIR/common.sh"
