#!/usr/bin/env bash
set -e

echo "Installing Go modules..."
go mod download

echo "Running tests..."
go test ./...

echo "Ready."
