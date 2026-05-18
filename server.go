package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type Server struct {
	mu       sync.Mutex
	sidecar  *Sidecar
	now      func() time.Time
	newID    func() string
	debounce time.Duration
	timer    *time.Timer
	mux      *http.ServeMux
}

type ServerOpts struct {
	Debounce time.Duration
	Now      func() time.Time
	NewID    func() string
}

func NewServer(sc *Sidecar, opts ServerOpts) *Server {
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if opts.NewID == nil {
		opts.NewID = randomNoteID
	}
	s := &Server{
		sidecar:  sc,
		now:      opts.Now,
		newID:    opts.NewID,
		debounce: opts.Debounce,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /annotations", s.list)
	mux.HandleFunc("POST /annotations", s.create)
	mux.HandleFunc("PUT /annotations/{id}", s.update)
	mux.HandleFunc("DELETE /annotations/{id}", s.delete)
	s.mux = mux
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) list(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"notes": s.sidecar.Notes})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Anchor Anchor `json:"anchor"`
		Body   string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	n := Note{
		ID:        s.newID(),
		Anchor:    in.Anchor,
		Body:      in.Body,
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	}
	s.sidecar.Notes = append(s.sidecar.Notes, n)
	s.scheduleFlush()
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Body     *string `json:"body"`
		Resolved *bool   `json:"resolved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.sidecar.Notes {
		if s.sidecar.Notes[i].ID != id {
			continue
		}
		if in.Body != nil {
			s.sidecar.Notes[i].Body = *in.Body
		}
		if in.Resolved != nil {
			s.sidecar.Notes[i].Resolved = *in.Resolved
		}
		s.scheduleFlush()
		writeJSON(w, http.StatusOK, s.sidecar.Notes[i])
		return
	}
	writeError(w, http.StatusNotFound, fmt.Errorf("note %q not found", id))
}

func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.sidecar.Notes {
		if s.sidecar.Notes[i].ID != id {
			continue
		}
		s.sidecar.Notes = append(s.sidecar.Notes[:i], s.sidecar.Notes[i+1:]...)
		s.scheduleFlush()
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeError(w, http.StatusNotFound, fmt.Errorf("note %q not found", id))
}

// scheduleFlush persists the sidecar. With debounce > 0 it coalesces rapid
// writes into a single disk flush after the quiet period. Callers must hold s.mu.
func (s *Server) scheduleFlush() {
	if s.debounce <= 0 {
		if err := s.sidecar.Save(); err != nil {
			fmt.Fprintln(os.Stderr, "peek: save sidecar:", err)
		}
		return
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(s.debounce, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.sidecar.Save(); err != nil {
			fmt.Fprintln(os.Stderr, "peek: save sidecar:", err)
		}
	})
}

// Flush cancels any pending debounced write and persists immediately.
func (s *Server) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if err := s.sidecar.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "peek: save sidecar:", err)
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func randomNoteID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("n_%d", time.Now().UnixNano())
	}
	return "n_" + hex.EncodeToString(b[:])
}
