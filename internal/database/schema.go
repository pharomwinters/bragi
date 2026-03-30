package database

// migrations is an ordered list of SQL statements, one per schema version.
// Each entry is applied inside a transaction. The index + 1 is the version number.
var migrations = []string{
	// Version 1: initial schema.
	`
	CREATE TABLE IF NOT EXISTS notes (
		id           INTEGER PRIMARY KEY,
		rel_path     TEXT    UNIQUE NOT NULL,
		title        TEXT    NOT NULL DEFAULT '',
		content_hash TEXT    NOT NULL,
		updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS chunks (
		id          INTEGER PRIMARY KEY,
		note_id     INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		chunk_index INTEGER NOT NULL,
		content     TEXT    NOT NULL,
		heading     TEXT    NOT NULL DEFAULT '',
		start_line  INTEGER NOT NULL,
		end_line    INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_chunks_note_id ON chunks(note_id);

	CREATE VIRTUAL TABLE IF NOT EXISTS vec_chunks USING vec0(
		chunk_id INTEGER PRIMARY KEY,
		embedding float[768]
	);

	CREATE TABLE IF NOT EXISTS wikilinks (
		id          INTEGER PRIMARY KEY,
		source_id   INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		target_name TEXT    NOT NULL,
		alias       TEXT    NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_wikilinks_source  ON wikilinks(source_id);
	CREATE INDEX IF NOT EXISTS idx_wikilinks_target  ON wikilinks(target_name);

	CREATE TABLE IF NOT EXISTS frontmatter (
		id      INTEGER PRIMARY KEY,
		note_id INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		key     TEXT    NOT NULL,
		value   TEXT    NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_frontmatter_note ON frontmatter(note_id);
	CREATE INDEX IF NOT EXISTS idx_frontmatter_kv   ON frontmatter(key, value);

	CREATE TABLE IF NOT EXISTS schema_version (
		version    INTEGER  PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	`,
}
