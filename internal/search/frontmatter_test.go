package search

import (
	"testing"
)

func TestParseFrontmatterSimple(t *testing.T) {
	raw := `title: "My Note"
author: Alice`
	entries := ParseFrontmatter(raw)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	found := make(map[string]string)
	for _, e := range entries {
		found[e.Key] = e.Value
	}

	if found["title"] != "My Note" {
		t.Errorf("title = %q, want 'My Note'", found["title"])
	}
	if found["author"] != "Alice" {
		t.Errorf("author = %q, want 'Alice'", found["author"])
	}
}

func TestParseFrontmatterArray(t *testing.T) {
	raw := `tags:
  - philosophy
  - ethics
  - logic`
	entries := ParseFrontmatter(raw)

	tagCount := 0
	for _, e := range entries {
		if e.Key == "tags" {
			tagCount++
		}
	}

	if tagCount != 3 {
		t.Errorf("expected 3 tag entries, got %d", tagCount)
	}
}

func TestParseFrontmatterNested(t *testing.T) {
	raw := `author:
  name: Alice
  email: alice@example.com`
	entries := ParseFrontmatter(raw)

	found := make(map[string]string)
	for _, e := range entries {
		found[e.Key] = e.Value
	}

	if found["author.name"] != "Alice" {
		t.Errorf("author.name = %q, want 'Alice'", found["author.name"])
	}
	if found["author.email"] != "alice@example.com" {
		t.Errorf("author.email = %q, want 'alice@example.com'", found["author.email"])
	}
}

func TestParseFrontmatterDate(t *testing.T) {
	raw := `created: 2025-01-15`
	entries := ParseFrontmatter(raw)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// YAML parses dates; we store as string representation.
	if entries[0].Key != "created" {
		t.Errorf("key = %q, want 'created'", entries[0].Key)
	}
	if entries[0].Value == "" {
		t.Error("expected non-empty date value")
	}
}

func TestParseFrontmatterEmpty(t *testing.T) {
	entries := ParseFrontmatter("")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty input, got %d", len(entries))
	}
}

func TestParseFrontmatterInvalid(t *testing.T) {
	entries := ParseFrontmatter("not: valid: yaml: {{{}}")
	// Should not panic, may return nil or empty.
	_ = entries
}
