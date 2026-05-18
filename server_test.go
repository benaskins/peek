package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md.peek.json")
	sc, err := LoadSidecar(path)
	if err != nil {
		t.Fatalf("LoadSidecar: %v", err)
	}
	sc.Source = "doc.md"

	srv := NewServer(sc, ServerOpts{
		Debounce: 0,
		Now:      func() time.Time { return time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC) },
		NewID:    sequentialIDs(),
	})
	return srv, path
}

func sequentialIDs() func() string {
	i := 0
	return func() string {
		i++
		return "n_test_" + string(rune('a'+i-1))
	}
}

func doJSON(t *testing.T, h http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func decodeNote(t *testing.T, rr *httptest.ResponseRecorder) Note {
	t.Helper()
	var n Note
	if err := json.Unmarshal(rr.Body.Bytes(), &n); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rr.Body.String())
	}
	return n
}

func TestServer_GetAnnotations_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := doJSON(t, srv, http.MethodGet, "/annotations", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Notes []Note `json:"notes"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Notes) != 0 {
		t.Errorf("Notes len = %d, want 0", len(resp.Notes))
	}
}

func TestServer_PostAnnotation_CreatesNote(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := doJSON(t, srv, http.MethodPost, "/annotations", map[string]any{
		"anchor": map[string]string{
			"block_hash": "abcd1234",
			"block_text": "Install via npm",
			"block_type": "paragraph",
		},
		"body": "Mention pnpm too",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	n := decodeNote(t, rr)
	if n.ID != "n_test_a" {
		t.Errorf("ID = %q, want %q", n.ID, "n_test_a")
	}
	if n.Body != "Mention pnpm too" {
		t.Errorf("Body = %q", n.Body)
	}
	if n.Anchor.BlockHash != "abcd1234" {
		t.Errorf("Anchor = %+v", n.Anchor)
	}
	if n.CreatedAt != "2026-05-19T10:00:00Z" {
		t.Errorf("CreatedAt = %q", n.CreatedAt)
	}
	if n.Resolved {
		t.Error("new note should not be resolved")
	}
}

func TestServer_PostThenGet_RoundTrip(t *testing.T) {
	srv, _ := newTestServer(t)
	doJSON(t, srv, http.MethodPost, "/annotations", map[string]any{
		"anchor": map[string]string{"block_hash": "x", "block_text": "y", "block_type": "paragraph"},
		"body":   "note one",
	})

	rr := doJSON(t, srv, http.MethodGet, "/annotations", nil)
	var resp struct {
		Notes []Note `json:"notes"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Notes) != 1 || resp.Notes[0].Body != "note one" {
		t.Errorf("got notes = %+v", resp.Notes)
	}
}

func TestServer_PostPersistsToDisk(t *testing.T) {
	srv, path := newTestServer(t)
	doJSON(t, srv, http.MethodPost, "/annotations", map[string]any{
		"anchor": map[string]string{"block_hash": "x", "block_text": "y", "block_type": "paragraph"},
		"body":   "persisted",
	})

	reloaded, err := LoadSidecar(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reloaded.Notes) != 1 || reloaded.Notes[0].Body != "persisted" {
		t.Errorf("reloaded = %+v", reloaded.Notes)
	}
}

func TestServer_PutUpdatesBodyAndResolved(t *testing.T) {
	srv, _ := newTestServer(t)
	create := doJSON(t, srv, http.MethodPost, "/annotations", map[string]any{
		"anchor": map[string]string{"block_hash": "x", "block_text": "y", "block_type": "paragraph"},
		"body":   "original",
	})
	id := decodeNote(t, create).ID

	rr := doJSON(t, srv, http.MethodPut, "/annotations/"+id, map[string]any{
		"body":     "edited",
		"resolved": true,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	n := decodeNote(t, rr)
	if n.Body != "edited" || !n.Resolved {
		t.Errorf("got %+v", n)
	}
}

func TestServer_PutUnknownID(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := doJSON(t, srv, http.MethodPut, "/annotations/nope", map[string]any{"body": "x"})
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestServer_Delete(t *testing.T) {
	srv, path := newTestServer(t)
	create := doJSON(t, srv, http.MethodPost, "/annotations", map[string]any{
		"anchor": map[string]string{"block_hash": "x", "block_text": "y", "block_type": "paragraph"},
		"body":   "to delete",
	})
	id := decodeNote(t, create).ID

	rr := doJSON(t, srv, http.MethodDelete, "/annotations/"+id, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rr.Code, rr.Body.String())
	}

	reloaded, _ := LoadSidecar(path)
	if len(reloaded.Notes) != 0 {
		t.Errorf("reloaded notes len = %d, want 0", len(reloaded.Notes))
	}
}

func TestServer_DeleteUnknownID(t *testing.T) {
	srv, _ := newTestServer(t)
	rr := doJSON(t, srv, http.MethodDelete, "/annotations/nope", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestServer_PostMalformedJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/annotations", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestServer_DebouncedWriteCoalesces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md.peek.json")
	sc, _ := LoadSidecar(path)
	srv := NewServer(sc, ServerOpts{
		Debounce: 25 * time.Millisecond,
		Now:      func() time.Time { return time.Unix(0, 0).UTC() },
		NewID:    sequentialIDs(),
	})

	for i := 0; i < 5; i++ {
		doJSON(t, srv, http.MethodPost, "/annotations", map[string]any{
			"anchor": map[string]string{"block_hash": "x", "block_text": "y", "block_type": "paragraph"},
			"body":   "rapid",
		})
	}

	srv.Flush()

	reloaded, err := LoadSidecar(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reloaded.Notes) != 5 {
		t.Errorf("reloaded notes len = %d, want 5", len(reloaded.Notes))
	}
}
