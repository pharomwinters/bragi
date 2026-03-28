// Package markdown provides CommonMark parsing with frontmatter extraction.
package markdown

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// Document represents a parsed markdown document.
type Document struct {
	// Frontmatter is the raw YAML frontmatter string (without delimiters).
	// Empty if the document has no frontmatter.
	Frontmatter string

	// Body is the markdown content after the frontmatter.
	Body string

	// Title extracted from frontmatter or first heading.
	Title string
}

// Parser wraps a configured goldmark instance.
type Parser struct {
	md goldmark.Markdown
}

// NewParser creates a markdown parser with CommonMark + extensions.
func NewParser() *Parser {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // tables, strikethrough, task lists, autolinks
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // allow raw HTML passthrough
		),
	)
	return &Parser{md: md}
}

// Parse splits a markdown document into frontmatter and body,
// and extracts metadata.
func (p *Parser) Parse(source string) Document {
	doc := Document{}

	// Extract YAML frontmatter.
	fm, body := extractFrontmatter(source)
	doc.Frontmatter = fm
	doc.Body = body

	// Extract title from frontmatter or first heading.
	doc.Title = extractTitle(fm, body)

	return doc
}

// RenderHTML renders markdown body to HTML.
func (p *Parser) RenderHTML(markdownBody string) (string, error) {
	var buf bytes.Buffer
	src := []byte(markdownBody)
	if err := p.md.Convert(src, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// WordCount returns the approximate word count of a markdown body,
// excluding frontmatter.
func WordCount(body string) int {
	// Simple split on whitespace. Good enough for status bar display.
	words := strings.Fields(body)
	return len(words)
}

// HeadingCount returns the number of headings in a markdown body.
func HeadingCount(body string) int {
	p := goldmark.DefaultParser()
	reader := text.NewReader([]byte(body))
	node := p.Parse(reader)

	count := 0
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if child.Kind().String() == "Heading" {
			count++
		}
	}
	return count
}

// extractFrontmatter splits YAML frontmatter from markdown body.
// Frontmatter must start at the very beginning of the document with "---".
func extractFrontmatter(source string) (frontmatter, body string) {
	if !strings.HasPrefix(source, "---") {
		return "", source
	}

	// Find closing delimiter.
	rest := source[3:]
	// Skip the newline after opening ---
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	closeIdx := strings.Index(rest, "\n---")
	if closeIdx == -1 {
		// No closing delimiter found — treat entire doc as body.
		return "", source
	}

	fm := rest[:closeIdx]
	body = rest[closeIdx+4:] // skip "\n---"

	// Skip newline after closing ---
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
		body = body[2:]
	}

	return strings.TrimSpace(fm), body
}

// extractTitle tries to get a title from frontmatter, falling back to
// the first ATX heading in the body.
func extractTitle(frontmatter, body string) string {
	// Try frontmatter: look for title: "value" or title: value
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "title:") {
			val := strings.TrimPrefix(line, "title:")
			val = strings.TrimSpace(val)
			// Strip quotes.
			val = strings.Trim(val, `"'`)
			if val != "" {
				return val
			}
		}
	}

	// Fall back to first heading.
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}

	return ""
}
