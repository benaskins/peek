# peek annotation mode — design

**Date:** 2026-05-18
**Status:** Design agreed, ready to implement

## Goal

Allow the user to annotate a rendered markdown page in the browser during an in-person review session, with notes persisting across `peek` invocations on the same file so they can be acted on later.

## Workflow

User sits with someone, walks through `README.md`, captures corrections/notes as they go. Later, opens the same file in their editor with `peek README.md` running alongside to work through the notes.

## Architecture

`peek` shifts from a one-shot CLI to a small local HTTP server that stays alive for the duration of the review session.

- `peek README.md` starts an HTTP server on a random port on 127.0.0.1, opens the browser, stays alive until Ctrl+C.
- Multiple concurrent `peek` instances on the same file: each starts its own server. Last write to the sidecar wins. Documented limitation, revisit if it bites.

## Storage

Sidecar JSON file next to the source markdown: `README.md.peek.json`.

- Visible (so users can `.gitignore` it if desired)
- Moves with `git mv`
- Debounced write (~300ms) on every annotation change

Format:

```json
{
  "version": 1,
  "source": "README.md",
  "notes": [
    {
      "id": "n_01HXYZ...",
      "anchor": {
        "block_hash": "sha1:abcd...",
        "block_text": "Install via npm…",
        "block_type": "paragraph"
      },
      "body": "Mention pnpm too",
      "resolved": false,
      "created_at": "2026-05-18T10:14:00Z"
    }
  ]
}
```

## Anchoring

Block-level anchoring (paragraph, heading, list item, blockquote, code block).

- Each rendered block gets a stable DOM id: `<div id="peek-<hash>">`, where `hash` is a hash of the block's normalized text content (whitespace collapsed, trimmed).
- Collision handling: salt the hash with the previous heading's text so two identical "Examples" headings under different parents are distinguishable.
- Implemented via a goldmark renderer hook (wrap each block-level node during render).

### Orphan handling

If a stored note's block_hash doesn't match any block on re-render, the note is "orphaned":

- Shown in a separate "Orphaned notes" section of the sidebar
- Original quoted block_text is shown for context
- User can manually re-anchor (by clicking a block while the orphan note is selected), delete, or leave as a standalone checklist item

**Deferred:** phrase-level (text-range) anchoring within a block. Too brittle for v1.

## UI

**Right sidebar**, collapsible, ~320px wide.

- Lists all notes grouped by the nearest preceding heading
- Each note: quoted block-text preview, body, timestamp, resolved checkbox, edit/delete buttons
- Orphaned notes in a separate section
- Resolved notes dim / collapse but remain visible (untick to bring back)

**Inline trigger:**

- Hover a block → small "💬+" marker in the left margin
- Click to open a note input attached to that block
- Annotated blocks show a filled marker

## Implementation impact

Codebase grows from ~165 LOC single file to roughly 400–600 LOC across:

- `main.go` — CLI entry, server lifecycle, signal handling
- `render.go` — markdown → HTML with block IDs (goldmark renderer customization)
- `store.go` — sidecar read/write, debouncing
- `server.go` — HTTP handlers (`GET /`, `GET /annotations`, `POST /annotations`, `PUT /annotations/:id`, `DELETE /annotations/:id`)
- `web/` — vanilla JS + CSS for the annotation client, embedded via `//go:embed`

No frontend framework. Keep dependencies tight.

## Deferred (explicitly out of scope for v1)

- Live reload of the source `.md` on file change
- Phrase-level / text-range anchoring within a block
- Multiple concurrent peek instances coordinating on the same file
- Auth / sharing / collaboration

## Implementation steps

Each step ends with a clean commit. Backend steps (1–4) are TDD; frontend steps (6–9) are manually verified in the browser.

1. **Block ID generation** — pure function `blockID(headingPath []string, text string) string`. Tests: deterministic, whitespace-normalized, heading-salted to disambiguate duplicate block text under different sections.
2. **Markdown rendering with block IDs** — goldmark renderer hook wraps each block-level node in `<div id="peek-<hash>" data-peek-block>…</div>`. Tests: sample inputs → HTML contains expected IDs; nested structures (lists, blockquotes) handled.
3. **Sidecar store** — `LoadSidecar(path)`, `Sidecar.Save()`. Tests: missing file → empty sidecar, save+reload round-trip, malformed JSON surfaced as error.
4. **HTTP server (annotation CRUD)** — `GET /annotations`, `POST /annotations`, `PUT /annotations/:id`, `DELETE /annotations/:id`. Tests with `httptest`. Debounced disk writes.
5. **CLI wiring + server lifecycle** — replace one-shot render with: start server on random port, open browser, wait for SIGINT. Smoke test by running peek manually.
6. **Frontend shell + sidebar** — embedded JS/CSS via `//go:embed`. Sidebar fetches `/annotations`, renders list grouped by section. Manual verify.
7. **Frontend: add note** — hover marker on each block, click → input → POST. Manual verify.
8. **Frontend: edit / delete / resolve** — note actions wired to server. Manual verify.
9. **Frontend: orphan section** — notes whose `block_hash` isn't in the current DOM render under "Orphaned notes." Manual verify.

## Open questions resolved

| Question | Decision |
|---|---|
| Persist annotations across sessions? | Yes |
| Annotations stick to doc on re-open? | Yes |
| Annotation anchor granularity? | Block (A) |
| Sidebar vs. margin layout? | Right sidebar |
| Resolved/done checkbox? | Yes |
| Storage location? | Sidecar `README.md.peek.json` |
