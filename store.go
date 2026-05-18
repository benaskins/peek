package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type Sidecar struct {
	Path    string `json:"-"`
	Version int    `json:"version"`
	Source  string `json:"source"`
	Notes   []Note `json:"notes"`
}

type Note struct {
	ID        string `json:"id"`
	Anchor    Anchor `json:"anchor"`
	Body      string `json:"body"`
	Resolved  bool   `json:"resolved"`
	CreatedAt string `json:"created_at"`
}

type Anchor struct {
	BlockHash string `json:"block_hash"`
	BlockText string `json:"block_text"`
	BlockType string `json:"block_type"`
}

func LoadSidecar(path string) (*Sidecar, error) {
	s := &Sidecar{Path: path, Version: 1, Notes: []Note{}}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read sidecar: %w", err)
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parse sidecar %s: %w", path, err)
	}
	s.Path = path
	if s.Notes == nil {
		s.Notes = []Note{}
	}
	return s, nil
}

func (s *Sidecar) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sidecar: %w", err)
	}

	dir := filepath.Dir(s.Path)
	tmp, err := os.CreateTemp(dir, ".peek-sidecar-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename sidecar: %w", err)
	}
	return nil
}
