package search

import (
	"context"
	"testing"
	"time"

	"github.com/adambick/bragi/internal/config"
	"github.com/adambick/bragi/internal/database"
	"github.com/adambick/bragi/internal/embedding"
	"github.com/adambick/bragi/internal/knowledgebase"
)

// mockProvider is a test embedding provider that returns fixed-dimension zero vectors.
type mockProvider struct {
	dims int
}

func (m *mockProvider) Embed(_ context.Context, texts []string) ([]embedding.Vector, error) {
	vecs := make([]embedding.Vector, len(texts))
	for i := range vecs {
		vecs[i] = make(embedding.Vector, m.dims)
		// Put a tiny distinguishing value so vectors aren't identical.
		if len(vecs[i]) > 0 {
			vecs[i][0] = float32(i) * 0.01
		}
	}
	return vecs, nil
}

func (m *mockProvider) EmbedQuery(_ context.Context, query string) (embedding.Vector, error) {
	v := make(embedding.Vector, m.dims)
	return v, nil
}

func (m *mockProvider) Dimensions() int { return m.dims }
func (m *mockProvider) Close() error    { return nil }

func setupTestIndexer(t *testing.T) (*Indexer, *database.DB) {
	t.Helper()

	db, err := database.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Create a temporary project.
	dir := t.TempDir()
	proj, err := knowledgebase.NewProject(dir, "test-project", "tester", "none")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	embedder := &mockProvider{dims: 768}
	cfg := config.SearchConfig{
		ChunkStrategy: "paragraph",
		IndexOnSave:   true,
	}

	idx := NewIndexer(db, embedder, proj, cfg)
	return idx, db
}

func TestIndexerSingleFile(t *testing.T) {
	idx, db := setupTestIndexer(t)
	defer db.Close()

	// Create a note in the project.
	relPath, err := idx.project.CreateNote("Test Note", "")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	content := `---
title: "Test Note"
tags: [test, demo]
---

# Introduction

This is a test note with enough words to form a reasonable chunk for embedding purposes and testing the indexer pipeline end to end.

## Second Section

Another section with some content to create multiple chunks in the paragraph strategy mode.`

	if err := idx.project.WriteNote(relPath, content); err != nil {
		t.Fatalf("WriteNote: %v", err)
	}

	// Start indexer and enqueue.
	idx.Start()
	idx.Enqueue(IndexRequest{RelPath: relPath})

	// Wait for processing.
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case status := <-idx.Status():
		if status.Err != nil {
			t.Fatalf("indexing error: %v", status.Err)
		}
	case <-timer.C:
		t.Fatal("timeout waiting for index status")
	}

	idx.Stop()

	// Verify note exists in DB.
	var noteCount int
	db.Pool().QueryRow("SELECT COUNT(*) FROM notes").Scan(&noteCount)
	if noteCount != 1 {
		t.Errorf("expected 1 note, got %d", noteCount)
	}

	// Verify chunks exist.
	var chunkCount int
	db.Pool().QueryRow("SELECT COUNT(*) FROM chunks").Scan(&chunkCount)
	if chunkCount == 0 {
		t.Error("expected chunks, got 0")
	}

	// Verify wikilinks (none in this doc, but table should be clean).
	var linkCount int
	db.Pool().QueryRow("SELECT COUNT(*) FROM wikilinks").Scan(&linkCount)

	// Verify frontmatter.
	var fmCount int
	db.Pool().QueryRow("SELECT COUNT(*) FROM frontmatter").Scan(&fmCount)
	if fmCount == 0 {
		t.Error("expected frontmatter entries, got 0")
	}
}

func TestIndexerSkipUnchanged(t *testing.T) {
	idx, db := setupTestIndexer(t)
	defer db.Close()

	relPath, err := idx.project.CreateNote("Skip Test", "")
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	content := "---\ntitle: Skip Test\n---\n\nSome content here with enough words to be meaningful."
	idx.project.WriteNote(relPath, content)

	idx.Start()

	// First index.
	idx.Enqueue(IndexRequest{RelPath: relPath})
	timer := time.NewTimer(5 * time.Second)
	select {
	case <-idx.Status():
	case <-timer.C:
		t.Fatal("timeout")
	}
	timer.Stop()

	// Second index of same content — should be skipped (hash match).
	idx.Enqueue(IndexRequest{RelPath: relPath})
	timer = time.NewTimer(5 * time.Second)
	select {
	case status := <-idx.Status():
		if status.Err != nil {
			t.Errorf("unexpected error on skip: %v", status.Err)
		}
	case <-timer.C:
		t.Fatal("timeout on second index")
	}
	timer.Stop()

	idx.Stop()

	// Still exactly 1 note.
	var count int
	db.Pool().QueryRow("SELECT COUNT(*) FROM notes").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 note, got %d", count)
	}
}

func TestIndexerDelete(t *testing.T) {
	idx, db := setupTestIndexer(t)
	defer db.Close()

	relPath, _ := idx.project.CreateNote("Delete Me", "")
	idx.project.WriteNote(relPath, "---\ntitle: Delete Me\n---\n\nContent to delete later.")

	idx.Start()

	// Index first.
	idx.Enqueue(IndexRequest{RelPath: relPath})
	timer := time.NewTimer(5 * time.Second)
	select {
	case <-idx.Status():
	case <-timer.C:
		t.Fatal("timeout")
	}
	timer.Stop()

	// Delete.
	idx.Enqueue(IndexRequest{RelPath: relPath, Delete: true})
	timer = time.NewTimer(5 * time.Second)
	select {
	case <-idx.Status():
	case <-timer.C:
		t.Fatal("timeout on delete")
	}
	timer.Stop()

	idx.Stop()

	var count int
	db.Pool().QueryRow("SELECT COUNT(*) FROM notes").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 notes after delete, got %d", count)
	}
}

func TestContentHash(t *testing.T) {
	h1 := contentHash("hello world")
	h2 := contentHash("hello world")
	h3 := contentHash("different content")

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(h1))
	}
}

func TestSerializeVector(t *testing.T) {
	v := embedding.Vector{1.0, 2.0, 3.0}
	buf := serializeVector(v)

	if len(buf) != 12 {
		t.Errorf("expected 12 bytes for 3 floats, got %d", len(buf))
	}
}
