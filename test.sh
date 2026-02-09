#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"

echo "Running tests..."
go test ./... -timeout 120s "$@"

echo "All tests passed."
