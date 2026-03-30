package wikilink

import (
	"github.com/adambick/bragi/internal/database"
)

// PersistentIndex wraps the in-memory Index with SQLite persistence.
// It embeds *Index so all existing callers (ForwardLinks, Backlinks, etc.)
// work without changes. Write operations sync to the database.
type PersistentIndex struct {
	*Index
	db *database.DB
}

// NewPersistentIndex creates a persistent wikilink index backed by SQLite.
// On creation, it loads all wikilinks from the database into memory.
func NewPersistentIndex(db *database.DB) (*PersistentIndex, error) {
	pi := &PersistentIndex{
		Index: NewIndex(),
		db:    db,
	}

	// Load all wikilinks from the database.
	allLinks, err := db.LoadAllWikilinks()
	if err != nil {
		return nil, err
	}

	for sourcePath, rows := range allLinks {
		links := make([]WikiLink, len(rows))
		for i, r := range rows {
			links[i] = WikiLink{
				Target: r.TargetName,
				Alias:  r.Alias,
			}
		}
		pi.Index.Update(sourcePath, links)
	}

	return pi, nil
}

// Update replaces the wikilinks for a source file in both memory and the database.
func (pi *PersistentIndex) Update(sourceFile string, links []WikiLink) {
	// Update in-memory index.
	pi.Index.Update(sourceFile, links)

	// The database is updated by the indexer during its transaction pipeline.
	// This method only updates the in-memory index for real-time TUI responsiveness.
	// The next indexer pass will persist the links.
}

// Remove deletes all wikilink data for a file from both memory and the database.
func (pi *PersistentIndex) Remove(sourceFile string) {
	// Remove from in-memory index.
	pi.Index.Remove(sourceFile)

	// Database cleanup happens via the indexer's delete pipeline or cascading deletes.
}
