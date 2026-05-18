package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
)

func renderToString(t *testing.T, src string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := renderWithBlockIDs(goldmark.New(), []byte(src), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func wantWrapper(t *testing.T, html string, id string) {
	t.Helper()
	want := fmt.Sprintf(`<div id="peek-%s" data-peek-block>`, id)
	if !strings.Contains(html, want) {
		t.Errorf("expected wrapper %q in HTML, got:\n%s", want, html)
	}
}

func TestRender_WrapsHeadingAndParagraphs(t *testing.T) {
	html := renderToString(t, "# Hello\n\nFirst paragraph.\n\nSecond paragraph.\n")

	wantWrapper(t, html, blockID(nil, "Hello"))
	wantWrapper(t, html, blockID([]string{"Hello"}, "First paragraph."))
	wantWrapper(t, html, blockID([]string{"Hello"}, "Second paragraph."))
}

func TestRender_HeadingPathTracksNesting(t *testing.T) {
	html := renderToString(t, "# Intro\n\nIntro body.\n\n## Setup\n\nSetup body.\n")

	wantWrapper(t, html, blockID([]string{"Intro"}, "Intro body."))
	wantWrapper(t, html, blockID([]string{"Intro", "Setup"}, "Setup body."))
}

func TestRender_HeadingPathTruncatesOnShallowerHeading(t *testing.T) {
	src := "# A\n\n## B\n\nBody under B.\n\n# C\n\nBody under C.\n"
	html := renderToString(t, src)

	wantWrapper(t, html, blockID([]string{"A", "B"}, "Body under B."))
	wantWrapper(t, html, blockID([]string{"C"}, "Body under C."))
}

func TestRender_CodeBlockHashedByContent(t *testing.T) {
	html := renderToString(t, "```\nfmt.Println(\"hi\")\n```\n")
	wantWrapper(t, html, blockID(nil, "fmt.Println(\"hi\")"))
}

func TestRender_BlockquoteIsOneBlock(t *testing.T) {
	html := renderToString(t, "> quoted line one\n> quoted line two\n")
	wantWrapper(t, html, blockID(nil, "quoted line one quoted line two"))
}

func TestRender_PreservesInnerRendering(t *testing.T) {
	html := renderToString(t, "Just **bold** text.\n")
	if !strings.Contains(html, "<strong>bold</strong>") {
		t.Errorf("expected inline rendering to still work, got:\n%s", html)
	}
}
