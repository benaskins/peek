package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderPage_ContainsBlockIDsAndMarkdownBody(t *testing.T) {
	page, err := renderPage("README.md", []byte("# Hello\n\nFirst paragraph.\n"))
	if err != nil {
		t.Fatalf("renderPage: %v", err)
	}
	pageStr := string(page)

	if !strings.Contains(pageStr, "<title>README.md</title>") {
		t.Errorf("expected title from filename, got:\n%s", pageStr)
	}
	if !strings.Contains(pageStr, `data-peek-block`) {
		t.Error("expected block wrappers in page")
	}
	if !strings.Contains(pageStr, "<h1") || !strings.Contains(pageStr, "Hello") {
		t.Error("expected rendered heading")
	}
	if !strings.Contains(pageStr, "<p>First paragraph.</p>") {
		t.Error("expected rendered paragraph")
	}
}

func TestRenderPage_ExtractsMermaid(t *testing.T) {
	page, err := renderPage("README.md", []byte("```mermaid\ngraph LR\nA-->B\n```\n"))
	if err != nil {
		t.Fatalf("renderPage: %v", err)
	}
	pageStr := string(page)
	if !strings.Contains(pageStr, `class="mermaid"`) {
		t.Errorf("expected mermaid div, got:\n%s", pageStr)
	}
}

func TestPageHandler_ServesFileFreshOnEachRequest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte("# v1\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := newPageHandler(path)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(rr.Body.String(), "v1") {
		t.Errorf("expected v1 in first response, got:\n%s", rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html…", ct)
	}

	if err := os.WriteFile(path, []byte("# v2\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(rr2.Body.String(), "v2") {
		t.Errorf("expected v2 in second response after rewrite, got:\n%s", rr2.Body.String())
	}
}

func TestPageHandler_MissingFile_500(t *testing.T) {
	h := newPageHandler("/no/such/file.md")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}
