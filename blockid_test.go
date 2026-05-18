package main

import "testing"

func TestBlockID_Deterministic(t *testing.T) {
	a := blockID(nil, "Hello world")
	b := blockID(nil, "Hello world")
	if a != b {
		t.Fatalf("expected same hash, got %q vs %q", a, b)
	}
}

func TestBlockID_CollapsesWhitespace(t *testing.T) {
	want := blockID(nil, "Hello world")
	cases := []string{"Hello  world", "Hello\tworld", "Hello\nworld", "Hello \t world"}
	for _, c := range cases {
		if got := blockID(nil, c); got != want {
			t.Errorf("blockID(%q) = %q, want %q", c, got, want)
		}
	}
}

func TestBlockID_Trims(t *testing.T) {
	want := blockID(nil, "Hello world")
	if got := blockID(nil, "  Hello world\n"); got != want {
		t.Errorf("blockID with surrounding whitespace = %q, want %q", got, want)
	}
}

func TestBlockID_CaseSensitive(t *testing.T) {
	if blockID(nil, "Hello") == blockID(nil, "hello") {
		t.Fatal("expected different hashes for different cases")
	}
}

func TestBlockID_HeadingSaltDistinguishesDuplicates(t *testing.T) {
	a := blockID([]string{"Intro"}, "Examples")
	b := blockID([]string{"Setup"}, "Examples")
	if a == b {
		t.Fatal("expected different hashes for duplicate text under different headings")
	}
}

func TestBlockID_NestedHeadingPath(t *testing.T) {
	a := blockID([]string{"Setup", "Linux"}, "Run make")
	b := blockID([]string{"Setup", "Mac"}, "Run make")
	if a == b {
		t.Fatal("expected different hashes at deeper heading levels")
	}
}

func TestBlockID_NilEqualsEmpty(t *testing.T) {
	if blockID(nil, "Hello") != blockID([]string{}, "Hello") {
		t.Fatal("nil and empty heading path should produce the same hash")
	}
}

func TestBlockID_ShortHex(t *testing.T) {
	h := blockID(nil, "anything")
	if len(h) != 12 {
		t.Fatalf("expected 12-char hash, got %q (len %d)", h, len(h))
	}
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("expected lowercase hex chars only, got %q", h)
		}
	}
}
