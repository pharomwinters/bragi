package search

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"sync"

	"golang.org/x/crypto/blake2b"

	"github.com/adambick/bragi/internal/config"
	"github.com/adambick/bragi/internal/database"
	"github.com/adambick/bragi/internal/embedding"
	"github.com/adambick/bragi/internal/knowledgebase"
	"github.com/adambick/bragi/internal/markdown"
	"github.com/adambick/bragi/internal/wikilink"
)

// IndexRequest is a unit of work for the background indexer.
type IndexRequest struct {
	RelPath string
	Content string // if empty, indexer reads from disk
	Delete  bool   // true = remove from index
}

// IndexStatus reports progress from the indexer.
type IndexStatus struct {
	RelPath   string
	Done      bool
	Err       error
	Message   string
	QueueSize int
}

// Indexer processes files in the background: chunks, embeds, and stores in SQLite.
type Indexer struct {
	db       *database.DB
	embedder embedding.Provider
	project  *knowledgebase.Project
	config   config.SearchConfig
	parser   *markdown.Parser

	requests chan IndexRequest
	status   chan IndexStatus
	quit     chan struct{}
	wg       sync.WaitGroup
}

// NewIndexer creates a new background indexer.
func NewIndexer(
	db *database.DB,
	embedder embedding.Provider,
	project *knowledgebase.Project,
	cfg config.SearchConfig,
) *Indexer {
	return &Indexer{
		db:       db,
		embedder: embedder,
		project:  project,
		config:   cfg,
		parser:   markdown.NewParser(),
		requests: make(chan IndexRequest, 1000),
		status:   make(chan IndexStatus, 100),
		quit:     make(chan struct{}),
	}
}

// Start begins the background worker goroutine.
func (idx *Indexer) Start() {
	idx.wg.Add(1)
	go idx.worker()
}

// Stop signals the worker to finish and waits for it to complete.
func (idx *Indexer) Stop() {
	close(idx.quit)
	idx.wg.Wait()
}

// Enqueue adds a file to the indexing queue.
// Never blocks — drops the request if the queue is full.
func (idx *Indexer) Enqueue(req IndexRequest) {
	// Check exclusion list.
	for _, dir := range idx.config.ExcludeDirectories {
		if strings.HasPrefix(req.RelPath, dir+"/") || req.RelPath == dir {
			return
		}
	}

	select {
	case idx.requests <- req:
	default:
		// Queue full, drop silently.
	}
}

// Status returns a read-only channel for receiving indexing progress.
func (idx *Indexer) Status() <-chan IndexStatus {
	return idx.status
}

// ReindexAll enqueues all markdown files in the project for indexing.
func (idx *Indexer) ReindexAll() error {
	files, err := idx.project.ListMarkdownFiles()
	if err != nil {
		return fmt.Errorf("listing files: %w", err)
	}

	for _, f := range files {
		idx.Enqueue(IndexRequest{RelPath: f})
	}

	return nil
}

// worker is the main processing loop.
func (idx *Indexer) worker() {
	defer idx.wg.Done()

	for {
		select {
		case <-idx.quit:
			return
		case req := <-idx.requests:
			remaining := len(idx.requests)

			if req.Delete {
				idx.deleteFile(req.RelPath, remaining)
			} else {
				idx.indexFile(req, remaining)
			}
		}
	}
}

// indexFile processes a single file through the full indexing pipeline.
func (idx *Indexer) indexFile(req IndexRequest, queueSize int) {
	content := req.Content
	if content == "" {
		var err error
		content, err = idx.project.ReadNote(req.RelPath)
		if err != nil {
			idx.emitStatus(req.RelPath, fmt.Errorf("reading file: %w", err), queueSize)
			return
		}
	}

	// 1. Hash content for change detection.
	hash := contentHash(content)

	// Check if already up to date.
	var existingHash string
	err := idx.db.Pool().QueryRow(
		"SELECT content_hash FROM notes WHERE rel_path = ?", req.RelPath,
	).Scan(&existingHash)
	if err == nil && existingHash == hash {
		idx.emitStatus(req.RelPath, nil, queueSize)
		return
	}

	// 2. Parse document.
	doc := idx.parser.Parse(content)

	// 3. Chunk body.
	chunks := ChunkDocument(doc.Body, idx.config.ChunkStrategy)

	// 4. Embed chunks.
	var vectors []embedding.Vector
	if len(chunks) > 0 && idx.embedder != nil {
		texts := make([]string, len(chunks))
		for i, c := range chunks {
			texts[i] = c.Content
		}

		vectors, err = idx.embedder.Embed(context.Background(), texts)
		if err != nil {
			idx.emitStatus(req.RelPath, fmt.Errorf("embedding: %w", err), queueSize)
			return
		}
	}

	// 5. Extract wikilinks.
	links := wikilink.Extract(doc.Body)

	// 6. Parse frontmatter.
	fmEntries := ParseFrontmatter(doc.Frontmatter)

	// 7. Write everything in a single transaction.
	err = idx.db.Tx(func(tx *sql.Tx) error {
		// Upsert note.
		var noteID int64
		err := tx.QueryRow(
			"SELECT id FROM notes WHERE rel_path = ?", req.RelPath,
		).Scan(&noteID)

		if err == sql.ErrNoRows {
			result, err := tx.Exec(
				"INSERT INTO notes (rel_path, title, content_hash) VALUES (?, ?, ?)",
				req.RelPath, doc.Title, hash,
			)
			if err != nil {
				return fmt.Errorf("inserting note: %w", err)
			}
			noteID, _ = result.LastInsertId()
		} else if err != nil {
			return fmt.Errorf("querying note: %w", err)
		} else {
			_, err = tx.Exec(
				"UPDATE notes SET title = ?, content_hash = ?, updated_at = datetime('now') WHERE id = ?",
				doc.Title, hash, noteID,
			)
			if err != nil {
				return fmt.Errorf("updating note: %w", err)
			}
		}

		// Delete old vector data first (references chunks).
		tx.Exec(
			"DELETE FROM vec_chunks WHERE chunk_id IN (SELECT id FROM chunks WHERE note_id = ?)", noteID,
		)

		// Delete old data for this note.
		if _, err := tx.Exec("DELETE FROM chunks WHERE note_id = ?", noteID); err != nil {
			return fmt.Errorf("deleting old chunks: %w", err)
		}
		if _, err := tx.Exec("DELETE FROM wikilinks WHERE source_id = ?", noteID); err != nil {
			return fmt.Errorf("deleting old wikilinks: %w", err)
		}
		if _, err := tx.Exec("DELETE FROM frontmatter WHERE note_id = ?", noteID); err != nil {
			return fmt.Errorf("deleting old frontmatter: %w", err)
		}

		// Insert chunks and embeddings.
		for i, chunk := range chunks {
			result, err := tx.Exec(
				"INSERT INTO chunks (note_id, chunk_index, content, heading, start_line, end_line) VALUES (?, ?, ?, ?, ?, ?)",
				noteID, chunk.Index, chunk.Content, chunk.Heading, chunk.StartLine, chunk.EndLine,
			)
			if err != nil {
				return fmt.Errorf("inserting chunk %d: %w", i, err)
			}

			if i < len(vectors) {
				chunkID, _ := result.LastInsertId()
				_, err = tx.Exec(
					"INSERT INTO vec_chunks (chunk_id, embedding) VALUES (?, ?)",
					chunkID, serializeVector(vectors[i]),
				)
				if err != nil {
					return fmt.Errorf("inserting embedding %d: %w", i, err)
				}
			}
		}

		// Insert wikilinks.
		for _, link := range links {
			_, err := tx.Exec(
				"INSERT INTO wikilinks (source_id, target_name, alias) VALUES (?, ?, ?)",
				noteID, wikilink.NormalizeTarget(link.Target), link.Alias,
			)
			if err != nil {
				return fmt.Errorf("inserting wikilink: %w", err)
			}
		}

		// Insert frontmatter.
		for _, entry := range fmEntries {
			_, err := tx.Exec(
				"INSERT INTO frontmatter (note_id, key, value) VALUES (?, ?, ?)",
				noteID, entry.Key, entry.Value,
			)
			if err != nil {
				return fmt.Errorf("inserting frontmatter: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		idx.emitStatus(req.RelPath, fmt.Errorf("transaction: %w", err), queueSize)
		return
	}

	idx.emitStatus(req.RelPath, nil, queueSize)
}

// deleteFile removes all data for a file from the database.
func (idx *Indexer) deleteFile(relPath string, queueSize int) {
	err := idx.db.Tx(func(tx *sql.Tx) error {
		// vec_chunks are cleaned up via the chunks cascade.
		// First delete vec_chunks for this note's chunks.
		_, err := tx.Exec(
			`DELETE FROM vec_chunks WHERE chunk_id IN
			 (SELECT c.id FROM chunks c JOIN notes n ON c.note_id = n.id WHERE n.rel_path = ?)`,
			relPath,
		)
		if err != nil {
			return err
		}

		// Delete the note (cascades to chunks, wikilinks, frontmatter).
		_, err = tx.Exec("DELETE FROM notes WHERE rel_path = ?", relPath)
		return err
	})

	idx.emitStatus(relPath, err, queueSize)
}

func (idx *Indexer) emitStatus(relPath string, err error, queueSize int) {
	msg := "Indexed"
	if err != nil {
		msg = err.Error()
	}

	select {
	case idx.status <- IndexStatus{
		RelPath:   relPath,
		Done:      err == nil,
		Err:       err,
		Message:   msg,
		QueueSize: queueSize,
	}:
	default:
		// Status channel full, drop.
	}
}

// contentHash computes a blake2b-256 hash of the content.
func contentHash(content string) string {
	h, _ := blake2b.New256(nil)
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

// serializeVector converts a float32 slice to little-endian bytes for sqlite-vec.
func serializeVector(v embedding.Vector) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}
