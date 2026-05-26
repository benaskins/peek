# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is peek

peek is a local-only markdown previewer with inline annotations. Run `peek doc.md` and it opens a browser tab showing the rendered markdown with a notes sidebar. Notes are persisted in a sidecar JSON file (`<file>.peek.json`) next to the source. The server binds to a random port on localhost and auto-shuts down when the browser tab closes (heartbeat/pagehide lifecycle).

## Build & Test

```bash
just build          # go build -o peek .
just test           # go test ./...
just install        # build + install to ~/.local/bin/peek
go test -run TestX  # run a single test
go test -v ./...    # verbose output
```

## Architecture

Single Go module (`package main`), flat layout. Frontend is vanilla JS/CSS embedded via `go:embed`.

**Request flow:** `main.go` wires the HTTP mux → page handler renders markdown on each request (no caching) → annotation API reads/writes the sidecar store → frontend JS drives the sidebar and inline forms.

Key pieces:

- **render.go / page.go** — Goldmark pipeline: parses markdown, extracts mermaid blocks pre-render, wraps each top-level AST node in a `<div id="peek-{hash}" data-peek-block>` wrapper. The hash is the block's content-addressable ID used to anchor notes.
- **blockid.go** — Computes the block hash: SHA-1 of the heading path (ancestor headings) + normalized block text, truncated to 12 hex chars. This is what ties a note to a specific block even if surrounding content changes.
- **store.go** — `Sidecar` / `Note` / `Anchor` types. Atomic writes via temp-file + rename.
- **server.go** — REST API for annotations (GET/POST/PUT/DELETE on `/annotations`). Debounced flush coalesces rapid writes. `ServerOpts` injects `Now` and `NewID` for deterministic tests.
- **watcher.go** — Browser lifecycle: `Beat()` resets the idle timer, `Bye()` fires immediate shutdown. The grace window (default 30s) covers brief network hiccups.
- **web/** — `peek.js` and `peek.css`, embedded into the binary. The JS fetches `/annotations`, renders the sidebar, injects per-block `+` markers, and handles create/edit/delete/resolve flows. Also runs the heartbeat interval and sends `/bye` on `pagehide`.

**Testing pattern:** Each Go source file has a `_test.go` companion. Server tests use `httptest.NewServer` with injected clocks and ID generators — no real disk or timers in tests.
