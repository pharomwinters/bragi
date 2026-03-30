package wikilink

import (
	"testing"

	"github.com/adambick/bragi/internal/database"
)

func setupTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func insertTestNote(t *testing.T, db *database.DB, relPath string) int64 {
	t.Helper()
	result, err := db.Pool().Exec(
		"INSERT INTO notes (rel_path, title, content_hash) VALUES (?, ?, ?)",
		relPath, "Test", "hash123",
	)
	if err != nil {
		t.Fatalf("insert note: %v", err)
	}
	id, _ := result.LastInsertId()
	return id
}

func TestPersistentIndexLoadFromDB(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert test data.
	noteID := insertTestNote(t, db, "note-a.md")

	db.Pool().Exec(
		"INSERT INTO wikilinks (source_id, target_name, alias) VALUES (?, ?, ?)",
		noteID, "target-b", "",
	)
	db.Pool().Exec(
		"INSERT INTO wikilinks (source_id, target_name, alias) VALUES (?, ?, ?)",
		noteID, "target-c", "alias-c",
	)

	// Create persistent index — should load from DB.
	pi, err := NewPersistentIndex(db)
	if err != nil {
		t.Fatalf("NewPersistentIndex: %v", err)
	}

	links := pi.ForwardLinks("note-a.md")
	if len(links) != 2 {
		t.Errorf("expected 2 forward links, got %d", len(links))
	}

	files, totalLinks, targets := pi.Stats()
	if files != 1 {
		t.Errorf("expected 1 file, got %d", files)
	}
	if totalLinks != 2 {
		t.Errorf("expected 2 links, got %d", totalLinks)
	}
	if targets != 2 {
		t.Errorf("expected 2 targets, got %d", targets)
	}
}

func TestPersistentIndexUpdate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pi, err := NewPersistentIndex(db)
	if err != nil {
		t.Fatalf("NewPersistentIndex: %v", err)
	}

	// Update in memory.
	links := []WikiLink{
		{Target: "foo", Alias: ""},
		{Target: "bar", Alias: "baz"},
	}
	pi.Update("test.md", links)

	// Verify in-memory state.
	got := pi.ForwardLinks("test.md")
	if len(got) != 2 {
		t.Errorf("expected 2 links, got %d", len(got))
	}

	backlinks := pi.Backlinks("foo")
	if len(backlinks) != 1 || backlinks[0] != "test.md" {
		t.Errorf("unexpected backlinks: %v", backlinks)
	}
}

func TestPersistentIndexRemove(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pi, err := NewPersistentIndex(db)
	if err != nil {
		t.Fatalf("NewPersistentIndex: %v", err)
	}

	pi.Update("remove-me.md", []WikiLink{{Target: "linked"}})
	pi.Remove("remove-me.md")

	links := pi.ForwardLinks("remove-me.md")
	if len(links) != 0 {
		t.Errorf("expected 0 links after remove, got %d", len(links))
	}

	backlinks := pi.Backlinks("linked")
	if len(backlinks) != 0 {
		t.Errorf("expected 0 backlinks after remove, got %d", len(backlinks))
	}
}
