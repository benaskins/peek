package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"

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

func main() {
	flag.Usage = func() { fmt.Fprintf(os.Stderr, "usage: peek <file.md>\n") }
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	path := flag.Arg(0)
	src, err := os.ReadFile(path)
	if err != nil {
		die("%v", err)
	}
	src = extractMermaid(src)

	md := goldmark.New(
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

	var body bytes.Buffer
	if err := md.Convert(src, &body); err != nil {
		die("render: %v", err)
	}

	light, err := chromaCSS("github")
	if err != nil {
		die("chroma css: %v", err)
	}
	dark, err := chromaCSS("github-dark")
	if err != nil {
		die("chroma css: %v", err)
	}

	page := fmt.Sprintf(pageTmpl, html.EscapeString(filepath.Base(path)), light, dark, body.String())

	out, err := os.CreateTemp("", "peek-*.html")
	if err != nil {
		die("tempfile: %v", err)
	}
	if _, err := out.WriteString(page); err != nil {
		die("write: %v", err)
	}
	out.Close()

	if err := openBrowser(out.Name()); err != nil {
		fmt.Fprintln(os.Stderr, "peek: open:", err)
		fmt.Println(out.Name())
		os.Exit(1)
	}
}

var (
	mermaidRE   = regexp.MustCompile("(?m)^```mermaid[ \\t]*\\r?\\n([\\s\\S]*?)\\r?\\n```[ \\t]*$")
	blankLineRE = regexp.MustCompile("(?m)\\n[ \\t]*\\n")
)

func extractMermaid(src []byte) []byte {
	return mermaidRE.ReplaceAllFunc(src, func(match []byte) []byte {
		m := mermaidRE.FindSubmatch(match)
		// Collapse blank lines so the <div> stays a single CommonMark HTML block.
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

func openBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Start()
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "peek: "+format+"\n", args...)
	os.Exit(1)
}
