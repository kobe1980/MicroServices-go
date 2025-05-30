#!/usr/bin/env bash
# Setup script for MicroServices projects
# Installs Go, Node.js, and RabbitMQ

set -euo pipefail

# Detect OS
if [[ $(uname) != "Linux" ]]; then
    echo "This script currently supports Debian/Ubuntu Linux only" >&2
    exit 1
fi

sudo apt-get update
sudo apt-get install -y curl gnupg

# Install Go 1.21
if ! command -v go >/dev/null; then
    curl -LO https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
    rm go1.21.0.linux-amd64.tar.gz
    echo "export PATH=\$PATH:/usr/local/go/bin" >> "$HOME/.profile"
fi

# Install Node.js (for reference implementation)
if ! command -v node >/dev/null; then
    curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
    sudo apt-get install -y nodejs
fi

# Install RabbitMQ
if ! command -v rabbitmq-server >/dev/null; then
    sudo apt-get install -y rabbitmq-server
    sudo systemctl enable rabbitmq-server
    sudo systemctl start rabbitmq-server
fi

# Download Go module dependencies for Go implementation
if [ -f go.mod ]; then
    go mod download
fi

# Install Node dependencies for JS implementation
if [ -d ../MicroServices ]; then
    (cd ../MicroServices && npm install)
fi

echo "Setup completed. Please restart your shell to use the new Go installation if it was installed."
