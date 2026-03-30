package search

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/adambick/bragi/internal/database"
	"github.com/adambick/bragi/internal/embedding"
)

// SearchResult represents a single search hit.
type SearchResult struct {
	RelPath   string
	Title     string
	ChunkText string  // the matched chunk content
	Heading   string  // section heading context
	Score     float32 // similarity score 0–1
	StartLine int
	EndLine   int
}

// Engine performs semantic search over the indexed knowledge base.
type Engine struct {
	db       *database.DB
	embedder embedding.Provider
}

// NewEngine creates a new search engine.
func NewEngine(db *database.DB, embedder embedding.Provider) *Engine {
	return &Engine{db: db, embedder: embedder}
}

// Search performs a semantic search with the given query.
func (e *Engine) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	return e.SearchWithFilter(ctx, query, nil, limit)
}

// SearchWithFilter performs a semantic search with optional frontmatter filters.
// Filters are key-value pairs that must match entries in the frontmatter table.
func (e *Engine) SearchWithFilter(ctx context.Context, query string, filters map[string]string, limit int) ([]SearchResult, error) {
	if e.embedder == nil {
		return nil, fmt.Errorf("embedding provider not available")
	}

	if limit <= 0 {
		limit = 10
	}

	// Embed the query.
	queryVec, err := e.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}

	queryBytes := serializeVector(queryVec)

	// Build the SQL query. We request more results than limit to allow
	// for deduplication by note.
	fetchLimit := limit * 3
	if fetchLimit < 20 {
		fetchLimit = 20
	}

	baseQuery := `
		SELECT c.content, c.heading, c.start_line, c.end_line,
		       n.rel_path, n.title, v.distance
		FROM vec_chunks v
		JOIN chunks c ON c.id = v.chunk_id
		JOIN notes n ON n.id = c.note_id
		WHERE v.embedding MATCH ? AND k = ?
	`

	args := []interface{}{queryBytes, fetchLimit}

	// Add frontmatter filters.
	if len(filters) > 0 {
		for k, val := range filters {
			baseQuery += " AND n.id IN (SELECT note_id FROM frontmatter WHERE key = ? AND value = ?)"
			args = append(args, k, val)
		}
	}

	baseQuery += " ORDER BY v.distance ASC"

	rows, err := e.db.Pool().QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	// Collect results, deduplicating by note (keep best chunk per note).
	seen := make(map[string]bool)
	var results []SearchResult

	for rows.Next() {
		var r SearchResult
		var distance float32

		if err := rows.Scan(&r.ChunkText, &r.Heading, &r.StartLine, &r.EndLine,
			&r.RelPath, &r.Title, &distance); err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}

		// Convert L2 distance to similarity score.
		// For L2-normalized vectors, distance is in [0, 2].
		// Similarity = 1 - distance/2, giving a range of [0, 1].
		r.Score = 1.0 - distance/2.0

		if seen[r.RelPath] {
			continue
		}
		seen[r.RelPath] = true

		results = append(results, r)
		if len(results) >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating results: %w", err)
	}

	return results, nil
}

// FindSimilar finds notes similar to the given note.
func (e *Engine) FindSimilar(ctx context.Context, relPath string, limit int) ([]SearchResult, error) {
	if e.embedder == nil {
		return nil, fmt.Errorf("embedding provider not available")
	}

	// Get the average embedding for this note's chunks.
	avgVec, err := e.noteAverageEmbedding(relPath)
	if err != nil {
		return nil, fmt.Errorf("computing average embedding: %w", err)
	}
	if avgVec == nil {
		return nil, nil // note not indexed
	}

	queryBytes := serializeVector(avgVec)

	// Search, excluding the source note.
	fetchLimit := limit * 3
	if fetchLimit < 20 {
		fetchLimit = 20
	}

	rows, err := e.db.Pool().QueryContext(ctx, `
		SELECT c.content, c.heading, c.start_line, c.end_line,
		       n.rel_path, n.title, v.distance
		FROM vec_chunks v
		JOIN chunks c ON c.id = v.chunk_id
		JOIN notes n ON n.id = c.note_id
		WHERE v.embedding MATCH ? AND k = ?
		  AND n.rel_path != ?
		ORDER BY v.distance ASC
	`, queryBytes, fetchLimit, relPath)
	if err != nil {
		return nil, fmt.Errorf("similar search: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var results []SearchResult

	for rows.Next() {
		var r SearchResult
		var distance float32

		if err := rows.Scan(&r.ChunkText, &r.Heading, &r.StartLine, &r.EndLine,
			&r.RelPath, &r.Title, &distance); err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}

		r.Score = 1.0 - distance/2.0

		if seen[r.RelPath] {
			continue
		}
		seen[r.RelPath] = true

		results = append(results, r)
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// noteAverageEmbedding computes the mean embedding vector for all chunks of a note.
func (e *Engine) noteAverageEmbedding(relPath string) (embedding.Vector, error) {
	rows, err := e.db.Pool().Query(`
		SELECT v.embedding
		FROM vec_chunks v
		JOIN chunks c ON c.id = v.chunk_id
		JOIN notes n ON n.id = c.note_id
		WHERE n.rel_path = ?
	`, relPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dims := e.embedder.Dimensions()
	avg := make(embedding.Vector, dims)
	count := 0

	for rows.Next() {
		var rawBytes []byte
		if err := rows.Scan(&rawBytes); err != nil {
			return nil, err
		}

		vec := deserializeVector(rawBytes, dims)
		for i := range avg {
			avg[i] += vec[i]
		}
		count++
	}

	if count == 0 {
		return nil, nil
	}

	for i := range avg {
		avg[i] /= float32(count)
	}

	// L2 normalize the average.
	var norm float64
	for _, x := range avg {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range avg {
			avg[i] = float32(float64(avg[i]) / norm)
		}
	}

	return avg, nil
}

// deserializeVector converts raw bytes back to a float32 vector.
func deserializeVector(buf []byte, dims int) embedding.Vector {
	if len(buf) < dims*4 {
		return make(embedding.Vector, dims)
	}
	vec := make(embedding.Vector, dims)
	for i := 0; i < dims; i++ {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return vec
}

// NoteCount returns the total number of indexed notes.
func (e *Engine) NoteCount() (int, error) {
	var count int
	err := e.db.Pool().QueryRow("SELECT COUNT(*) FROM notes").Scan(&count)
	return count, err
}

// ChunkCount returns the total number of indexed chunks.
func (e *Engine) ChunkCount() (int, error) {
	var count int
	err := e.db.Pool().QueryRow("SELECT COUNT(*) FROM chunks").Scan(&count)
	return count, err
}
