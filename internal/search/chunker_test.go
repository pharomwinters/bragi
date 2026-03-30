package search

import (
	"strings"
	"testing"
)

func TestChunkByParagraphBasic(t *testing.T) {
	body := "First paragraph here.\n\nSecond paragraph here.\n\nThird paragraph here with more words to make it longer so it stands alone as a chunk that has enough content."

	chunks := ChunkDocument(body, "paragraph")
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// Verify all chunks have content.
	for i, c := range chunks {
		if strings.TrimSpace(c.Content) == "" {
			t.Errorf("chunk %d has empty content", i)
		}
		if c.Index != i {
			t.Errorf("chunk %d has Index=%d", i, c.Index)
		}
	}
}

func TestChunkByHeading(t *testing.T) {
	body := `Some intro text before any heading.

# First Section

Content of first section.

## Subsection

Content of subsection.

# Second Section

Content of second section.`

	chunks := ChunkDocument(body, "heading")
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(chunks))
	}

	// First chunk should be pre-heading content.
	if chunks[0].Heading != "" {
		t.Errorf("first chunk heading should be empty, got %q", chunks[0].Heading)
	}

	// Second chunk should have heading "First Section".
	found := false
	for _, c := range chunks {
		if c.Heading == "First Section" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a chunk with heading 'First Section'")
	}
}

func TestChunkMergesSmall(t *testing.T) {
	// Two very short paragraphs should be merged.
	body := "Short.\n\nAlso short."
	chunks := ChunkDocument(body, "paragraph")

	// Both are under minChunkWords, so they should merge into 1 chunk.
	if len(chunks) != 1 {
		t.Errorf("expected 1 merged chunk, got %d", len(chunks))
	}
}

func TestChunkSplitsLarge(t *testing.T) {
	// Build a chunk larger than maxChunkChars.
	var builder strings.Builder
	for i := 0; i < 300; i++ {
		builder.WriteString("This is a sentence. ")
	}
	body := builder.String()

	chunks := ChunkDocument(body, "paragraph")
	if len(chunks) < 2 {
		t.Errorf("expected chunk to be split, got %d chunks", len(chunks))
	}

	for _, c := range chunks {
		if len(c.Content) > maxChunkChars+200 { // allow some tolerance
			t.Errorf("chunk too large: %d chars", len(c.Content))
		}
	}
}

func TestChunkLineNumbers(t *testing.T) {
	body := "Line 1\n\nLine 3\nLine 4\n\nLine 6"
	chunks := ChunkDocument(body, "paragraph")

	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}

	// First chunk should start at line 1.
	if chunks[0].StartLine != 1 {
		t.Errorf("first chunk StartLine = %d, want 1", chunks[0].StartLine)
	}
}

func TestChunkHeadingContext(t *testing.T) {
	body := `# Introduction

Some intro text that is long enough to be a standalone chunk with many words filling the minimum requirement for chunk word counts in the system.

# Methods

Some methods text that is also long enough to be a standalone chunk with enough words to meet the minimum word count for a separate chunk.`

	chunks := ChunkDocument(body, "paragraph")
	foundIntro := false
	foundMethods := false
	for _, c := range chunks {
		if c.Heading == "Introduction" {
			foundIntro = true
		}
		if c.Heading == "Methods" {
			foundMethods = true
		}
	}

	if !foundIntro {
		t.Error("expected a chunk with heading 'Introduction'")
	}
	if !foundMethods {
		t.Error("expected a chunk with heading 'Methods'")
	}
}

func TestChunkEmptyDocument(t *testing.T) {
	chunks := ChunkDocument("", "paragraph")
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty doc, got %d", len(chunks))
	}

	chunks = ChunkDocument("   \n\n  ", "heading")
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for whitespace doc, got %d", len(chunks))
	}
}

func TestExtractHeading(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"# Hello", "Hello"},
		{"## Sub Heading", "Sub Heading"},
		{"### Deep", "Deep"},
		{"Not a heading", ""},
		{"#NoSpace", ""},
		{"", ""},
		{"###### Level 6", "Level 6"},
		{"####### Too Deep", ""},
	}

	for _, tt := range tests {
		got := extractHeading(tt.input)
		if got != tt.want {
			t.Errorf("extractHeading(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
