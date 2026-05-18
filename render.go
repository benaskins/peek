package main

import (
	"bytes"
	"fmt"
	"io"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func renderWithBlockIDs(md goldmark.Markdown, src []byte, w io.Writer) error {
	doc := md.Parser().Parse(text.NewReader(src))
	var headingPath []string

	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		text := blockText(c, src)
		id := blockID(headingPath, text)
		fmt.Fprintf(w, `<div id="peek-%s" data-peek-block>`, id)
		if err := md.Renderer().Render(w, src, c); err != nil {
			return err
		}
		io.WriteString(w, "</div>\n")
		if h, ok := c.(*ast.Heading); ok {
			headingPath = updateHeadingPath(headingPath, h.Level, text)
		}
	}
	return nil
}

func updateHeadingPath(path []string, level int, text string) []string {
	if level < 1 {
		return path
	}
	for len(path) > level-1 {
		path = path[:len(path)-1]
	}
	for len(path) < level-1 {
		path = append(path, "")
	}
	return append(path, text)
}

func blockText(n ast.Node, src []byte) string {
	var buf bytes.Buffer
	ast.Walk(n, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := node.(type) {
		case *ast.Text:
			buf.Write(v.Segment.Value(src))
			buf.WriteByte(' ')
		case *ast.FencedCodeBlock:
			appendBlockLines(&buf, v, src)
			return ast.WalkSkipChildren, nil
		case *ast.CodeBlock:
			appendBlockLines(&buf, v, src)
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return buf.String()
}

func appendBlockLines(buf *bytes.Buffer, n ast.Node, src []byte) {
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(src))
	}
}
