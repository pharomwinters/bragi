// Package database provides SQLite persistence for Bragi projects.
// It uses mattn/go-sqlite3 with the sqlite-vec extension for vector search.
package database

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // register base sqlite3 driver
)

const dbFileName = "bragi.db"

// DB wraps a SQLite connection pool configured for Bragi.
type DB struct {
	pool *sql.DB
	path string
}

// Open opens (or creates) the Bragi database in projectDir.
func Open(projectDir string) (*DB, error) {
	path := filepath.Join(projectDir, dbFileName)
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000&_synchronous=NORMAL", path)

	pool, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Verify the connection works and sqlite-vec is loaded.
	var vecVersion string
	if err := pool.QueryRow("SELECT vec_version()").Scan(&vecVersion); err != nil {
		pool.Close()
		return nil, fmt.Errorf("sqlite-vec not available: %w", err)
	}

	return &DB{pool: pool, path: path}, nil
}

// OpenMemory opens an in-memory database for testing.
func OpenMemory() (*DB, error) {
	dsn := "file::memory:?_foreign_keys=ON"
	pool, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("opening in-memory database: %w", err)
	}

	var vecVersion string
	if err := pool.QueryRow("SELECT vec_version()").Scan(&vecVersion); err != nil {
		pool.Close()
		return nil, fmt.Errorf("sqlite-vec not available: %w", err)
	}

	return &DB{pool: pool, path: ":memory:"}, nil
}

// Close closes the database connection pool.
func (db *DB) Close() error {
	return db.pool.Close()
}

// Pool returns the underlying sql.DB for direct queries.
func (db *DB) Pool() *sql.DB {
	return db.pool
}

// Path returns the filesystem path to the database file.
func (db *DB) Path() string {
	return db.path
}

// Migrate runs all pending schema migrations.
func (db *DB) Migrate() error {
	current, err := db.currentVersion()
	if err != nil {
		return fmt.Errorf("checking schema version: %w", err)
	}

	for i := current; i < len(migrations); i++ {
		if err := db.applyMigration(i+1, migrations[i]); err != nil {
			return fmt.Errorf("applying migration %d: %w", i+1, err)
		}
	}
	return nil
}

// Tx runs fn inside a transaction, committing on success or rolling back on error/panic.
func (db *DB) Tx(fn func(tx *sql.Tx) error) error {
	tx, err := db.pool.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// currentVersion returns the highest applied migration version, or 0 if none.
func (db *DB) currentVersion() (int, error) {
	// schema_version table may not exist yet.
	var name string
	err := db.pool.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='schema_version'",
	).Scan(&name)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	var version int
	err = db.pool.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	return version, err
}

// applyMigration runs a single migration inside a transaction.
func (db *DB) applyMigration(version int, ddl string) error {
	tx, err := db.pool.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(ddl); err != nil {
		return fmt.Errorf("executing DDL: %w", err)
	}

	if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", version); err != nil {
		return fmt.Errorf("recording version: %w", err)
	}

	return tx.Commit()
}
