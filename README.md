# peek

A local markdown previewer with inline annotations. Point it at a markdown file, and it opens a rendered view in your browser with a notes sidebar for leaving comments on individual blocks.

## Install

Requires Go 1.26+.

```bash
git clone https://github.com/benaskins/peek.git
cd peek
just install
```

This builds the binary and installs it to `~/.local/bin/peek`.

## Usage

```bash
peek doc.md
```

peek starts a local HTTP server on a random port, opens the page in your default browser, and shuts itself down when you close the tab — no cleanup needed.

## Notes

Hover over any block (paragraph, heading, code block, list, etc.) to reveal a `+` marker on the left margin. Click it to open an inline form. Notes appear in the sidebar, grouped by section heading.

Each note supports **edit**, **delete**, and **resolve** (dims it in the sidebar). Delete requires a confirmation click.

## Persistence

Notes are stored in a sidecar file alongside the source: `doc.md` gets `doc.md.peek.json`. The file is plain JSON and safe to commit, share, or delete.

Notes are anchored to blocks by a content hash, so they survive edits to other parts of the document. If a block's content changes enough that its hash no longer matches, the note moves to an **Orphaned** section in the sidebar rather than disappearing.

## Features

- GitHub-flavoured markdown (tables, task lists, strikethrough, autolinks)
- Footnotes
- Syntax-highlighted code blocks (Chroma, class-based — light/dark)
- Mermaid diagrams
- Dark mode (follows system preference)
- Live re-render on each page load (edit the file, refresh the browser)
