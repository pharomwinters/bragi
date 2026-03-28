package wikilink

import (
	"testing"
)

func TestExtract(t *testing.T) {
	text := `This note discusses [[Trust as Social Contract]] and also
references [[Rust Ownership Model]] and the concept of [[Trust|trust in systems]].
Empty links [[]] should be ignored.`

	links := Extract(text)

	if len(links) != 3 {
		t.Fatalf("expected 3 links, got %d: %+v", len(links), links)
	}

	if links[0].Target != "Trust as Social Contract" {
		t.Errorf("expected first target 'Trust as Social Contract', got %q", links[0].Target)
	}
	if links[0].Alias != "" {
		t.Errorf("expected no alias on first link, got %q", links[0].Alias)
	}

	if links[1].Target != "Rust Ownership Model" {
		t.Errorf("expected second target 'Rust Ownership Model', got %q", links[1].Target)
	}

	if links[2].Target != "Trust" {
		t.Errorf("expected third target 'Trust', got %q", links[2].Target)
	}
	if links[2].Alias != "trust in systems" {
		t.Errorf("expected alias 'trust in systems', got %q", links[2].Alias)
	}
}

func TestExtractEmpty(t *testing.T) {
	links := Extract("No wikilinks here.")
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d", len(links))
	}
}

func TestExtractDuplicates(t *testing.T) {
	text := "[[Foo]] and [[Foo]] again"
	links := Extract(text)
	if len(links) != 2 {
		t.Errorf("Extract should return all occurrences, got %d", len(links))
	}

	targets := Targets(text)
	if len(targets) != 1 {
		t.Errorf("Targets should deduplicate, got %d: %v", len(targets), targets)
	}
}

func TestTargets(t *testing.T) {
	text := "[[Alpha]] then [[Beta]] and [[alpha]] again"
	targets := Targets(text)
	// "alpha" and "Alpha" should deduplicate (case insensitive).
	if len(targets) != 2 {
		t.Errorf("expected 2 unique targets, got %d: %v", len(targets), targets)
	}
}

func TestNormalizeTarget(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Trust as Social Contract", "trust as social contract"},
		{"  Spaces  ", "spaces"},
		{"UPPER", "upper"},
	}
	for _, tt := range tests {
		got := NormalizeTarget(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeTarget(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIndex(t *testing.T) {
	idx := NewIndex()

	// File A links to B and C.
	linksA := Extract("See [[Note B]] and [[Note C]]")
	idx.Update("a.md", linksA)

	// File D also links to B.
	linksD := Extract("Also see [[Note B]]")
	idx.Update("d.md", linksD)

	// Forward links.
	fwd := idx.ForwardLinks("a.md")
	if len(fwd) != 2 {
		t.Errorf("expected 2 forward links for a.md, got %d", len(fwd))
	}

	// Backlinks for Note B — should come from a.md and d.md.
	back := idx.Backlinks("Note B")
	if len(back) != 2 {
		t.Errorf("expected 2 backlinks for Note B, got %d: %v", len(back), back)
	}

	// Backlinks for Note C — should come from a.md only.
	back = idx.Backlinks("Note C")
	if len(back) != 1 {
		t.Errorf("expected 1 backlink for Note C, got %d", len(back))
	}

	// Stats.
	files, links, targets := idx.Stats()
	if files != 2 || links != 3 || targets != 2 {
		t.Errorf("Stats = (%d, %d, %d), want (2, 3, 2)", files, links, targets)
	}

	// Update a.md to only link to C — B backlink from a.md should be removed.
	newLinksA := Extract("Now only [[Note C]]")
	idx.Update("a.md", newLinksA)

	back = idx.Backlinks("Note B")
	if len(back) != 1 {
		t.Errorf("after update, expected 1 backlink for Note B, got %d: %v", len(back), back)
	}

	// Remove d.md entirely.
	idx.Remove("d.md")
	back = idx.Backlinks("Note B")
	if len(back) != 0 {
		t.Errorf("after remove, expected 0 backlinks for Note B, got %d", len(back))
	}
}
