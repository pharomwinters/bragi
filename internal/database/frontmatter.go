package database

import (
	"fmt"
)

// QueryByFrontmatter returns all note relative paths where the given key matches the value.
func (db *DB) QueryByFrontmatter(key, value string) ([]string, error) {
	rows, err := db.pool.Query(`
		SELECT DISTINCT n.rel_path
		FROM frontmatter f
		JOIN notes n ON n.id = f.note_id
		WHERE f.key = ? AND f.value = ?
	`, key, value)
	if err != nil {
		return nil, fmt.Errorf("querying frontmatter: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}

	return paths, rows.Err()
}

// ListFrontmatterKeys returns all distinct frontmatter keys in the database.
func (db *DB) ListFrontmatterKeys() ([]string, error) {
	rows, err := db.pool.Query("SELECT DISTINCT key FROM frontmatter ORDER BY key")
	if err != nil {
		return nil, fmt.Errorf("listing keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}

	return keys, rows.Err()
}

// ListFrontmatterValues returns all distinct values for a given frontmatter key.
func (db *DB) ListFrontmatterValues(key string) ([]string, error) {
	rows, err := db.pool.Query(
		"SELECT DISTINCT value FROM frontmatter WHERE key = ? ORDER BY value", key,
	)
	if err != nil {
		return nil, fmt.Errorf("listing values: %w", err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}

	return values, rows.Err()
}
