package wikilink

import "sync"

// Index is an in-memory bidirectional wikilink index.
// It tracks which files link to which targets, enabling
// both forward link and backlink queries.
//
// Thread-safe for concurrent access.
type Index struct {
	mu sync.RWMutex

	// forwardLinks maps source file (relative path) to its outgoing wikilinks.
	forwardLinks map[string][]WikiLink

	// backlinks maps normalized target name to source files that link to it.
	backlinks map[string][]string
}

// NewIndex creates an empty wikilink index.
func NewIndex() *Index {
	return &Index{
		forwardLinks: make(map[string][]WikiLink),
		backlinks:    make(map[string][]string),
	}
}

// Update replaces the wikilinks for a given source file.
// Call this whenever a file is loaded or saved.
func (idx *Index) Update(sourceFile string, links []WikiLink) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Remove old backlink entries for this source.
	idx.removeBacklinks(sourceFile)

	// Store forward links.
	idx.forwardLinks[sourceFile] = links

	// Rebuild backlinks for this source.
	for _, l := range links {
		normalized := NormalizeTarget(l.Target)
		idx.backlinks[normalized] = append(idx.backlinks[normalized], sourceFile)
	}
}

// Remove deletes all index entries for a file (e.g., on file deletion).
func (idx *Index) Remove(sourceFile string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeBacklinks(sourceFile)
	delete(idx.forwardLinks, sourceFile)
}

// ForwardLinks returns all wikilinks in the given source file.
func (idx *Index) ForwardLinks(sourceFile string) []WikiLink {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.forwardLinks[sourceFile]
}

// Backlinks returns all files that link to the given target name.
func (idx *Index) Backlinks(targetName string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	normalized := NormalizeTarget(targetName)
	return idx.backlinks[normalized]
}

// AllTargets returns all unique normalized target names in the index.
func (idx *Index) AllTargets() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	targets := make([]string, 0, len(idx.backlinks))
	for t := range idx.backlinks {
		targets = append(targets, t)
	}
	return targets
}

// Stats returns basic index statistics.
func (idx *Index) Stats() (files int, links int, uniqueTargets int) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	files = len(idx.forwardLinks)
	for _, flinks := range idx.forwardLinks {
		links += len(flinks)
	}
	uniqueTargets = len(idx.backlinks)
	return
}

// removeBacklinks removes all backlink entries where sourceFile is the linker.
// Must be called with idx.mu held.
func (idx *Index) removeBacklinks(sourceFile string) {
	oldLinks := idx.forwardLinks[sourceFile]
	for _, l := range oldLinks {
		normalized := NormalizeTarget(l.Target)
		sources := idx.backlinks[normalized]
		filtered := sources[:0]
		for _, s := range sources {
			if s != sourceFile {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			delete(idx.backlinks, normalized)
		} else {
			idx.backlinks[normalized] = filtered
		}
	}
}
