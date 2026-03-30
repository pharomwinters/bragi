package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMemory(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	// Verify we can query.
	var result int
	if err := db.Pool().QueryRow("SELECT 1").Scan(&result); err != nil {
		t.Fatalf("basic query: %v", err)
	}
	if result != 1 {
		t.Fatalf("expected 1, got %d", result)
	}
}

func TestVecExtension(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	var version string
	if err := db.Pool().QueryRow("SELECT vec_version()").Scan(&version); err != nil {
		t.Fatalf("vec_version: %v", err)
	}
	if version == "" {
		t.Fatal("vec_version returned empty string")
	}
	t.Logf("sqlite-vec version: %s", version)
}

func TestMigrate(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Verify tables exist.
	tables := []string{"notes", "chunks", "wikilinks", "frontmatter", "schema_version"}
	for _, table := range tables {
		var name string
		err := db.Pool().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}

	// Verify vec_chunks virtual table.
	var name string
	err = db.Pool().QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='vec_chunks'",
	).Scan(&name)
	if err != nil {
		t.Errorf("virtual table vec_chunks not found: %v", err)
	}

	// Verify schema version recorded.
	var version int
	if err := db.Pool().QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version); err != nil {
		t.Fatalf("schema_version query: %v", err)
	}
	if version != len(migrations) {
		t.Errorf("expected version %d, got %d", len(migrations), version)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}

func TestNotesInsert(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	_, err = db.Pool().Exec(
		"INSERT INTO notes (rel_path, title, content_hash) VALUES (?, ?, ?)",
		"test.md", "Test Note", "abc123",
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	var title string
	if err := db.Pool().QueryRow("SELECT title FROM notes WHERE rel_path = ?", "test.md").Scan(&title); err != nil {
		t.Fatalf("query: %v", err)
	}
	if title != "Test Note" {
		t.Errorf("expected 'Test Note', got %q", title)
	}
}

func TestTransaction(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Tx that returns an error should roll back.
	err = db.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO notes (rel_path, title, content_hash) VALUES (?, ?, ?)",
			"rollback.md", "Rollback Test", "def456")
		if err != nil {
			return err
		}
		return fmt.Errorf("intentional error")
	})
	if err == nil {
		t.Fatal("expected error from Tx")
	}

	// Verify the row was not committed.
	var count int
	db.Pool().QueryRow("SELECT COUNT(*) FROM notes WHERE rel_path = ?", "rollback.md").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}

	// Successful Tx should commit.
	err = db.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO notes (rel_path, title, content_hash) VALUES (?, ?, ?)",
			"committed.md", "Committed", "ghi789")
		return err
	})
	if err != nil {
		t.Fatalf("successful Tx: %v", err)
	}

	db.Pool().QueryRow("SELECT COUNT(*) FROM notes WHERE rel_path = ?", "committed.md").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 row after commit, got %d", count)
	}
}

func TestOpenFile(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Verify file was created.
	dbPath := filepath.Join(dir, dbFileName)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("database file not created at %s", dbPath)
	}

	if db.Path() != dbPath {
		t.Errorf("Path() = %q, want %q", db.Path(), dbPath)
	}
}
