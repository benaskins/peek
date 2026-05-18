package main

import (
	"bytes"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	htmlrenderer "github.com/yuin/goldmark/renderer/html"
)

const pageTmpl = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/github-markdown-css@5/github-markdown.min.css">
<style>
  body { box-sizing: border-box; min-width: 200px; max-width: 980px; margin: 0 auto; padding: 45px; background: #ffffff; color: #1f2328; }
  @media (max-width: 767px) { body { padding: 15px; } }
  @media (prefers-color-scheme: dark) {
    body { background: #0d1117; color: #e6edf3; }
  }
  .markdown-body .mermaid { background: transparent; padding: 0; margin: 16px 0; text-align: center; overflow: visible; }
  .markdown-body .mermaid svg { max-width: 100%%; height: auto; }
%s
  @media (prefers-color-scheme: dark) {
%s
  }
</style>
</head>
<body>
<article class="markdown-body">
%s
</article>
<script type="module">
  import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs';
  const dark = matchMedia('(prefers-color-scheme: dark)').matches;
  mermaid.initialize({ startOnLoad: true, theme: dark ? 'dark' : 'default' });
</script>
</body>
</html>
`

var (
	mermaidRE   = regexp.MustCompile("(?m)^```mermaid[ \\t]*\\r?\\n([\\s\\S]*?)\\r?\\n```[ \\t]*$")
	blankLineRE = regexp.MustCompile("(?m)\\n[ \\t]*\\n")
)

func renderPage(path string, src []byte) ([]byte, error) {
	src = extractMermaid(src)

	md := newGoldmark()
	var body bytes.Buffer
	if err := renderWithBlockIDs(md, src, &body); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	light, err := chromaCSS("github")
	if err != nil {
		return nil, err
	}
	dark, err := chromaCSS("github-dark")
	if err != nil {
		return nil, err
	}

	page := fmt.Sprintf(pageTmpl, html.EscapeString(filepath.Base(path)), light, dark, body.String())
	return []byte(page), nil
}

func newPageHandler(path string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		src, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		page, err := renderPage(path, src)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	})
}

func newGoldmark() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(htmlrenderer.WithUnsafe()),
	)
}

func extractMermaid(src []byte) []byte {
	return mermaidRE.ReplaceAllFunc(src, func(match []byte) []byte {
		m := mermaidRE.FindSubmatch(match)
		diagram := blankLineRE.ReplaceAllString(string(m[1]), "\n")
		for blankLineRE.MatchString(diagram) {
			diagram = blankLineRE.ReplaceAllString(diagram, "\n")
		}
		return []byte("<div class=\"mermaid\">\n" + html.EscapeString(diagram) + "\n</div>")
	})
}

func chromaCSS(styleName string) (string, error) {
	style := styles.Get(styleName)
	if style == nil {
		return "", fmt.Errorf("unknown chroma style: %s", styleName)
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	var buf bytes.Buffer
	if err := formatter.WriteCSS(&buf, style); err != nil {
		return "", err
	}
	return buf.String(), nil
}
