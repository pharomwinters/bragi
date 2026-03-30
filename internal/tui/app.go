package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/adambick/bragi/internal/database"
	"github.com/adambick/bragi/internal/embedding"
	"github.com/adambick/bragi/internal/knowledgebase"
	"github.com/adambick/bragi/internal/search"
	"github.com/adambick/bragi/internal/theme"
	"github.com/adambick/bragi/internal/wikilink"
)

// modelDownloadProgress writes a simple byte-count progress line to stderr.
// Called from initSearch before the TUI alt-screen activates.
func modelDownloadProgress(dir string) chan<- embedding.DownloadProgress {
	ch := make(chan embedding.DownloadProgress, 32)
	go func() {
		lastFile := ""
		for p := range ch {
			if p.Done {
				fmt.Fprintf(os.Stderr, "\rDownload complete.                        \n")
				return
			}
			if p.Err != nil {
				fmt.Fprintf(os.Stderr, "\rDownload error: %v\n", p.Err)
				return
			}
			if p.File != lastFile {
				if lastFile != "" {
					fmt.Fprintf(os.Stderr, "\n")
				}
				fmt.Fprintf(os.Stderr, "  Downloading %s", p.File)
				lastFile = p.File
			}
			if p.TotalBytes > 0 {
				pct := 100 * p.BytesDownloaded / p.TotalBytes
				fmt.Fprintf(os.Stderr, "\r  Downloading %s … %d%%", p.File, pct)
			}
		}
	}()
	return ch
}

// Run launches the Bubble Tea TUI application.
func Run(proj *knowledgebase.Project) error {
	t := theme.ByName(proj.Config.Editor.Theme)

	// Phase 2: Initialize search infrastructure.
	// All components are optional — Bragi degrades gracefully.
	db, indexer, searchEng, linkIdx, cleanup := initSearch(proj)
	defer cleanup()

	m := NewModel(proj, t, db, indexer, searchEng, linkIdx)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err
}

// initSearch sets up the database, embedding provider, indexer, and search engine.
// Returns nil components if any step fails (graceful degradation).
// The cleanup function must be called on exit.
func initSearch(proj *knowledgebase.Project) (
	*database.DB,
	*search.Indexer,
	*search.Engine,
	*wikilink.PersistentIndex,
	func(),
) {
	noopCleanup := func() {}

	// 1. Open database.
	db, err := database.Open(proj.RootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: database unavailable: %v\n", err)
		return nil, nil, nil, nil, noopCleanup
	}

	if err := db.Migrate(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: database migration failed: %v\n", err)
		db.Close()
		return nil, nil, nil, nil, noopCleanup
	}

	// 2. Load persistent wikilink index from DB.
	linkIdx, err := wikilink.NewPersistentIndex(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: wikilink index load failed: %v\n", err)
		// Continue — DB is still usable.
		linkIdx = nil
	}

	// 3. Try to load embedding model.
	modelName := proj.Config.Search.EmbeddingModel
	if modelName == "" {
		modelName = "nomic-ai/nomic-embed-text-v1.5"
	}

	// Check whether the model is already cached so we can warn before the
	// alt-screen activates (stderr is readable while the terminal is normal).
	modelCacheDir, _ := embedding.ModelCacheDir(modelName)
	needsDownload := !embedding.ModelCached(modelCacheDir)
	if needsDownload {
		fmt.Fprintf(os.Stderr, "Embedding model not found in cache. Downloading %s…\n", modelName)
		fmt.Fprintf(os.Stderr, "(cache: %s)\n", modelCacheDir)
	}

	progress := modelDownloadProgress(modelCacheDir)
	modelDir, err := embedding.EnsureModel(modelName, progress)
	close(progress)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: embedding model download failed: %v\n"+
				"Semantic search will be unavailable.\n"+
				"Run 'bragi index --download-model' to retry.\n", err)
		// Return DB + linkIdx but no search.
		return db, nil, nil, linkIdx, func() { db.Close() }
	}

	embedder, err := embedding.NewONNXProvider(modelDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ONNX Runtime not available: %v\n", err)
		return db, nil, nil, linkIdx, func() { db.Close() }
	}

	// 4. Create indexer and search engine.
	indexer := search.NewIndexer(db, embedder, proj, proj.Config.Search)
	indexer.Start()

	// Trigger background reindex of all files.
	indexer.ReindexAll()

	searchEng := search.NewEngine(db, embedder)

	cleanup := func() {
		indexer.Stop()
		embedder.Close()
		db.Close()
	}

	return db, indexer, searchEng, linkIdx, cleanup
}
