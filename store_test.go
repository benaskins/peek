package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSidecar_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md.peek.json")

	s, err := LoadSidecar(path)
	if err != nil {
		t.Fatalf("LoadSidecar on missing file: %v", err)
	}
	if s == nil {
		t.Fatal("LoadSidecar returned nil sidecar")
	}
	if s.Path != path {
		t.Errorf("Path = %q, want %q", s.Path, path)
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if len(s.Notes) != 0 {
		t.Errorf("Notes len = %d, want 0", len(s.Notes))
	}
}

func TestSidecar_SaveAndReloadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md.peek.json")

	s, err := LoadSidecar(path)
	if err != nil {
		t.Fatalf("LoadSidecar: %v", err)
	}
	s.Source = "doc.md"
	s.Notes = []Note{
		{
			ID:   "n_01",
			Body: "Mention pnpm too",
			Anchor: Anchor{
				BlockHash: "abcd1234",
				BlockText: "Install via npm…",
				BlockType: "paragraph",
			},
			Resolved:  false,
			CreatedAt: "2026-05-18T10:14:00Z",
		},
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := LoadSidecar(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Source != "doc.md" {
		t.Errorf("Source = %q, want %q", reloaded.Source, "doc.md")
	}
	if len(reloaded.Notes) != 1 {
		t.Fatalf("Notes len = %d, want 1", len(reloaded.Notes))
	}
	got := reloaded.Notes[0]
	if got.ID != "n_01" || got.Body != "Mention pnpm too" {
		t.Errorf("unexpected note: %+v", got)
	}
	if got.Anchor.BlockHash != "abcd1234" || got.Anchor.BlockText != "Install via npm…" || got.Anchor.BlockType != "paragraph" {
		t.Errorf("anchor not round-tripped: %+v", got.Anchor)
	}
}

func TestLoadSidecar_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.md.peek.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := LoadSidecar(path); err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestSidecar_SaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md.peek.json")

	s, err := LoadSidecar(path)
	if err != nil {
		t.Fatalf("LoadSidecar: %v", err)
	}
	s.Source = "doc.md"
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only the sidecar file after Save, got %v", names)
	}
}
