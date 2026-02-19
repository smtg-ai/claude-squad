#!/usr/bin/env bash
set -e
cd "$(dirname "$0")"
go build -o cs . && exec ./cs "$@"
