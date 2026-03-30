package database

import (
	"fmt"
)

// WikilinkRow represents a wikilink stored in the database.
type WikilinkRow struct {
	SourcePath string
	TargetName string
	Alias      string
}

// LoadAllWikilinks returns all wikilinks keyed by source file path.
func (db *DB) LoadAllWikilinks() (map[string][]WikilinkRow, error) {
	rows, err := db.pool.Query(`
		SELECT n.rel_path, w.target_name, w.alias
		FROM wikilinks w
		JOIN notes n ON n.id = w.source_id
		ORDER BY n.rel_path
	`)
	if err != nil {
		return nil, fmt.Errorf("querying wikilinks: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]WikilinkRow)
	for rows.Next() {
		var r WikilinkRow
		if err := rows.Scan(&r.SourcePath, &r.TargetName, &r.Alias); err != nil {
			return nil, fmt.Errorf("scanning wikilink: %w", err)
		}
		result[r.SourcePath] = append(result[r.SourcePath], r)
	}

	return result, rows.Err()
}

// QueryBacklinks returns all source file paths that link to the given target.
func (db *DB) QueryBacklinks(targetName string) ([]string, error) {
	rows, err := db.pool.Query(`
		SELECT DISTINCT n.rel_path
		FROM wikilinks w
		JOIN notes n ON n.id = w.source_id
		WHERE w.target_name = ?
	`, targetName)
	if err != nil {
		return nil, fmt.Errorf("querying backlinks: %w", err)
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

// NoteIDByPath returns the database ID for a note at the given relative path.
func (db *DB) NoteIDByPath(relPath string) (int64, error) {
	var id int64
	err := db.pool.QueryRow("SELECT id FROM notes WHERE rel_path = ?", relPath).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("note not found: %s: %w", relPath, err)
	}
	return id, nil
}
