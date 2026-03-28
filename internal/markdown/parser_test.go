package markdown

import (
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	p := NewParser()

	source := `---
title: "Trust as Social Contract"
created: 2026-03-27
tags: [philosophy, trust]
---

# Trust as Social Contract

Some body text here.
`
	doc := p.Parse(source)

	if doc.Frontmatter == "" {
		t.Error("expected non-empty frontmatter")
	}
	if !strings.Contains(doc.Frontmatter, "Trust as Social Contract") {
		t.Error("frontmatter should contain title")
	}
	if doc.Title != "Trust as Social Contract" {
		t.Errorf("expected title 'Trust as Social Contract', got %q", doc.Title)
	}
	if !strings.Contains(doc.Body, "# Trust as Social Contract") {
		t.Error("body should contain heading")
	}
	if strings.Contains(doc.Body, "---") {
		t.Error("body should not contain frontmatter delimiters")
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	p := NewParser()

	source := `# Just a Heading

Body text without frontmatter.
`
	doc := p.Parse(source)

	if doc.Frontmatter != "" {
		t.Errorf("expected empty frontmatter, got %q", doc.Frontmatter)
	}
	if doc.Title != "Just a Heading" {
		t.Errorf("expected title from heading, got %q", doc.Title)
	}
	if doc.Body != source {
		t.Error("body should be entire source when no frontmatter")
	}
}

func TestParseFrontmatterTitleFormats(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "double quoted",
			source: "---\ntitle: \"Quoted Title\"\n---\n\nBody.",
			want:   "Quoted Title",
		},
		{
			name:   "single quoted",
			source: "---\ntitle: 'Single Quoted'\n---\n\nBody.",
			want:   "Single Quoted",
		},
		{
			name:   "unquoted",
			source: "---\ntitle: Unquoted Title\n---\n\nBody.",
			want:   "Unquoted Title",
		},
		{
			name:   "no title in frontmatter, falls back to heading",
			source: "---\nauthor: Alice\n---\n\n# Heading Title\n\nBody.",
			want:   "Heading Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := p.Parse(tt.source)
			if doc.Title != tt.want {
				t.Errorf("expected title %q, got %q", tt.want, doc.Title)
			}
		})
	}
}

func TestRenderHTML(t *testing.T) {
	p := NewParser()

	html, err := p.RenderHTML("**bold** and *italic*")
	if err != nil {
		t.Fatalf("RenderHTML failed: %v", err)
	}
	if !strings.Contains(html, "<strong>bold</strong>") {
		t.Errorf("expected <strong> tag, got: %s", html)
	}
	if !strings.Contains(html, "<em>italic</em>") {
		t.Errorf("expected <em> tag, got: %s", html)
	}
}

func TestRenderHTMLExtensions(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"strikethrough", "~~deleted~~", "<del>deleted</del>"},
		{"task list", "- [x] done\n- [ ] todo", `type="checkbox"`},
		{"table", "| a | b |\n|---|---|\n| 1 | 2 |", "<table>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := p.RenderHTML(tt.input)
			if err != nil {
				t.Fatalf("RenderHTML failed: %v", err)
			}
			if !strings.Contains(html, tt.contains) {
				t.Errorf("expected HTML to contain %q, got: %s", tt.contains, html)
			}
		})
	}
}

func TestWordCount(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello world", 2},
		{"one", 1},
		{"", 0},
		{"  spaced  out  text  ", 3},
		{"# Heading\n\nBody with five words here.", 7},
	}
	for _, tt := range tests {
		got := WordCount(tt.input)
		if got != tt.want {
			t.Errorf("WordCount(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
