#!/bin/bash

echo "Installing golangci-lint v2.5.0 with $(go version)..."
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0

if ! command -v yamllint &>/dev/null; then
    echo "yamllint could not be found! Installing..."

    if command -v brew &>/dev/null; then
        echo "Installing yamllint via Homebrew..."
        brew install yamllint
    elif command -v pipx &>/dev/null; then
        echo "Installing yamllint via pipx..."
        pipx install yamllint
    elif command -v pip &>/dev/null; then
        pip install --user yamllint
    elif command -v pip3 &>/dev/null; then
        pip3 install --user yamllint
    else
        echo "⚠️  Warning: No package manager found, skipping yamllint installation"
        echo "   Install manually: brew install yamllint (macOS) or pip install yamllint"
    fi
fi

if ! command -v buf &>/dev/null; then
    echo "buf could not be found! Installing..."

    go install github.com/bufbuild/buf/cmd/buf@latest
fi

echo "✅ Setup linting complete."
