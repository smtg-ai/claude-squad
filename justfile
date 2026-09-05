# armarquez/claude-squad — a fork of smtg-ai/claude-squad carrying Forgejo/tea CLI
# support (upstream only speaks GitHub/gh) as a small patch on top of upstream. These
# recipes exist to keep that patch easy to rebase forward as upstream moves.

default:
    @just --list

# One-time: point this fork at smtg-ai/claude-squad so `sync-upstream` has something to fetch
add-upstream:
    #!/usr/bin/env bash
    set -euo pipefail
    if git remote get-url upstream >/dev/null 2>&1; then
        echo "✓ upstream already set: $(git remote get-url upstream)"
    else
        git remote add upstream https://github.com/smtg-ai/claude-squad.git
        echo "✓ added upstream -> https://github.com/smtg-ai/claude-squad.git"
    fi

# Verify without changing anything: is this branch behind upstream/main?
check-upstream:
    #!/usr/bin/env bash
    set -euo pipefail
    if ! git remote get-url upstream >/dev/null 2>&1; then
        echo "✗ no upstream remote — run: just add-upstream"
        exit 1
    fi
    git fetch upstream --quiet
    behind=$(git rev-list --count HEAD..upstream/main)
    if [ "$behind" -eq 0 ]; then
        echo "✓ up to date with upstream/main"
    else
        echo "⚠ $behind commit(s) behind upstream/main — run: just sync-upstream"
    fi

# Rebase local Forgejo/tea patches on top of latest upstream/main. Does NOT push —
# review + resolve conflicts, then `git push --force-with-lease origin main` yourself.
sync-upstream: add-upstream
    git fetch upstream
    git rebase upstream/main

# Build the binary
build:
    go build ./...

# Run the test suite
test:
    go test ./...
