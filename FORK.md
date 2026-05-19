# FORK.md — BippleDops/claude-squad

This is a fork of [smtg-ai/claude-squad](https://github.com/smtg-ai/claude-squad)
maintained to support [Paperclip](https://github.com/BippleDops/paperclip),
a multi-agent orchestration backend that needs a programmatic execution
substrate for tmux-and-worktree-per-agent work.

## What diverges

**The upstream is unchanged where it matters.** The TUI, config format,
session state on disk, `tmux` + `git worktree` semantics, and `--autoyes`
daemon all behave exactly like upstream. Running `cs` with no
subcommand is byte-identical.

The fork **adds**:

1. **`cs serve` subcommand** (`cmd/serve.go` — in the upstream
   codebase's `main.go` command table) that starts a headless
   HTTP+SSE server exposing session lifecycle: create / list / get /
   pane / input / pause / resume / kill / diff / commit / events.
   The server and the TUI can't run in the same process but share
   the same `session/` package — they are peers, not alternatives.

2. **`server/` package** — HTTP handlers, SSE event bus, in-memory
   instance registry, JSON DTOs, bearer-token auth. Net-new
   directory; nothing else imports it.

3. **`otel/` package** (second commit) — OpenTelemetry tracer
   initialized at `cs serve` boot, exporting to an OTLP/HTTP
   endpoint (target: self-hosted Langfuse in Paperclip's deployment).
   Emits `cs.instance.*` spans and reads W3C `TRACEPARENT` from
   inbound requests to stitch into the calling orchestrator's trace.
   Also injects `TRACEPARENT` + the standard `OTEL_*` env bundle into
   the agent subprocess (`claude`, `codex`, `aider`, `gemini`, …) so
   those emit their own spans as children.

## Why fork instead of upstream-first

The HTTP API and OTEL are meaningful architectural surface area.
Shipping both as an upstream-first PR would stall behind design
discussion before Paperclip can validate the integration. The plan
is:

1. Ship the fork so Paperclip consumers (one operator, one homelab)
   can run against it *today*.
2. Open an **early draft PR** to `smtg-ai/claude-squad` with the same
   two commits so the upstream maintainers can argue with the API
   shape and the OTEL hooks independently. Each commit is mergeable
   on its own.
3. Rebase the fork's `paperclip` branch onto upstream weekly so
   divergence stays small.

## Branch layout

- `main` — mirrors `upstream/main`. Reset to upstream periodically.
- `paperclip` — working branch. All fork-specific work lives here.
- `upstream` remote points at `smtg-ai/claude-squad`.

Routine fork maintenance:

```bash
git fetch upstream
git checkout main && git merge upstream/main --ff-only
git checkout paperclip && git rebase main
git push --force-with-lease origin paperclip
```

## Binary compatibility

- Installing this fork produces a `cs` binary identical in existing
  command behavior.
- The new `cs serve` subcommand is additive — running without it
  leaves the application behaving exactly like upstream.
- No upstream flags, subcommands, or config fields change semantics.

## Reference

- Paperclip design spec that motivates the fork:
  <https://github.com/BippleDops/paperclip/blob/main/docs/superpowers/specs/2026-04-18-paperclip-claude-squad-and-langfuse-design.md>
- Paperclip's Langfuse OTEL groundwork (already live in Paperclip's
  `claude_local` harness; this fork is the Go-side analogue):
  <https://github.com/BippleDops/paperclip/blob/main/docs/superpowers/specs/2026-04-18-phase-5c-langfuse-notes.md>
- Upstream PR for API shape discussion (opened with this fork):
  see `README` on `BippleDops/claude-squad` for the live link.
