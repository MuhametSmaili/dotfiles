#!/usr/bin/env bash

install_pi_agent() {
    echo "📦 Installing Pi..."
    run_cmd npm install -g --prefix "$HOME/.local" @earendil-works/pi-coding-agent@latest
}
