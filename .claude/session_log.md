# Session Log

## 2026-03-27 — Phase 1 Core Engine MVP

### What was done

**Project setup:**

- Initialized Go module (`github.com/adambick/bragi`, Go 1.26.1)
- Added GPLv3 license and NOTICE with Dracula Theme credit to Zeno Rocha and Lucas de França
- Scaffolded project directory structure (`cmd/bragi/`, `internal/` with 11 packages)

**Phase 1 implementation (7 chunks, all complete):**

1. **Foundation** — Config structs with TOML parsing (`go-toml/v2`), validation, defaults. Dracula + Alucard theme definitions with full hex palettes. Cobra CLI skeleton.

2. **Knowledge Base** — `bragi new` creates project dirs with methodology-specific structure (PARA, Zettelkasten, Johnny Decimal). `bragi open` loads projects with upward directory discovery. Note CRUD (create, read, write, rename, delete).

3. **Markdown + Wikilinks** — Goldmark parser with GFM extensions (tables, strikethrough, task lists). YAML frontmatter extraction. Regex-based `[[wikilink]]` and `[[target|alias]]` extraction. In-memory bidirectional index (forward links + backlinks).

4. **TUI Shell** — Bubble Tea app with sidebar file tree (bubbles list), editor area, status bar. Terminal resize handling. Tab focus switching, Ctrl+B sidebar toggle.

5. **Markdown Editor** — Bubbles textarea with line numbers, Ctrl+S save, modified tracking, word count. Auto-focus on file selection from sidebar.

6. **Command Palette + File Ops** — Ctrl+P searchable command overlay. New Note, Rename, Delete, Save, Toggle Sidebar, Switch Theme commands. Modal dialogs for input and confirmation.

7. **Wikilink Integration + Find + Polish** — Wikilinks indexed on file load and save. Ctrl+F find bar with match counting. Unsaved changes warning on quit. Project-wide wikilink indexing on startup.

**Bug fix:** Theme switch via command palette now propagates to all child components (editor, file tree, status bar, palette, dialog, find bar). Previously only updated the root model's theme field — children kept their construction-time copies.

### Stats

- 22 Go source files, 3 test files
- 8 packages with code: config, theme, knowledgebase, markdown, wikilink, filetree, editor, tui
- 3 packages still empty (Phase 2): database, embedding, search
- 27 passing tests
- ~3,600 lines of Go

### What's next (Phase 2)

- SQLite database embedded in project directory (`bragi.db`)
- sqlite-vec for vector search
- Local embedding generation via ONNX Runtime
- Background indexing with goroutines
- Semantic search interface
- Wikilink cross-reference index in SQLite
- Frontmatter index for structured queries

---

## 2026-03-30 — Phase 2 Semantic Search & SQLite Persistence

**Summary:** Added semantic search, ONNX-based embedding generation, SQLite persistence, and integrated these features into the TUI and CLI.

**Goal:** Add semantic search and persistent storage via SQLite and ONNX-based embeddings.

**Phase 2 implementation (7 chunks, all complete):**

**Phase 2 implementation (7 chunks, all complete):**

1. **Database Layer** — SQLite + sqlite-vec via `mattn/go-sqlite3` and `asg017/sqlite-vec-go-bindings`. Schema: `notes`, `chunks`, `vec_chunks` (virtual), `wikilinks`, `frontmatter`, `schema_version`. WAL mode, foreign keys, versioned migrations. `Open()`, `OpenMemory()`, `Migrate()`, `Tx()` helpers. 7 passing tests.

2. **Document Chunking** — `ChunkDocument(body, strategy)` with paragraph and heading modes. Paragraph: split on `\n\n`, merge small adjacent chunks (<50 words), split oversized (>2000 chars) at sentence boundaries, never merge across heading boundaries. Heading: split on ATX headings. Each chunk tracks `Index`, `Content`, `Heading`, `StartLine`, `EndLine`. 8 passing tests.

3. **Embedding Generation** — `Provider` interface with `ONNXProvider` implementation using `yalue/onnxruntime_go`. BERT WordPiece tokenizer reading `vocab.txt`. Model download from HuggingFace to `~/.cache/bragi/models/`. Batched inference (max 32), mean pooling with attention mask, L2 normalization. Nomic asymmetric prefixes (`search_document:` / `search_query:`). 4 passing tests (unit; integration tests require ONNX Runtime installed).

4. **Background Indexer** — Goroutine worker with buffered channel (cap 1000). Pipeline: blake2b content hashing for change detection → markdown parse → chunk → embed → extract wikilinks → parse frontmatter → single SQLite transaction. `ReindexAll()` enqueues all project files. Respects `ExcludeDirectories` config. 5 passing tests, race-detector clean.

5. **Semantic Search Engine** — `Engine` with `Search()`, `SearchWithFilter()`, `FindSimilar()`. sqlite-vec `MATCH` query with k-NN. Frontmatter filter support. L2 distance → similarity conversion. Per-note deduplication. `NoteCount()` and `ChunkCount()` stats.

6. **Wikilink & Frontmatter Persistence** — `PersistentIndex` embeds `*wikilink.Index`, loads all wikilinks from SQLite on startup via `LoadAllWikilinks()`. `QueryBacklinks()`, `NoteIDByPath()`. Frontmatter: `QueryByFrontmatter()`, `ListFrontmatterKeys()`, `ListFrontmatterValues()`. YAML parsing via `gopkg.in/yaml.v3` with nested map flattening and array expansion. 3 persistent index tests + 6 frontmatter tests.

7. **TUI Integration** — Search overlay (`Ctrl+K`) with debounced query, result display (title, heading, snippet, similarity score), cursor navigation, Enter to open file. Added to command palette as "Semantic Search" and "Reindex All". Index progress displayed in status bar. `app.go` initializes DB → model → embedder → persistent wikilinks → indexer → search engine with graceful degradation (all nil-guarded). `bragi index` CLI subcommand for headless reindexing. `.gitignore` updated for `bragi.db*`.

### Stats

- 34 Go source files (was 22), 9 test files (was 3)
- 11 packages with code (was 8): added database, embedding, search
- 34 passing tests (was 27), race-detector clean
- ~6,200 lines of Go (was ~3,600)

### New dependencies

- `github.com/mattn/go-sqlite3` v1.14.38
- `github.com/asg017/sqlite-vec-go-bindings` v0.1.6
- `github.com/yalue/onnxruntime_go` v1.27.0
- `golang.org/x/crypto` v0.49.0
- `gopkg.in/yaml.v3` v3.0.1

### Build requirements

- `CGO_ENABLED=1` (required by mattn/go-sqlite3)
- ONNX Runtime shared library on system for embedding generation
- Model auto-downloads on first use (~100MB to `~/.cache/bragi/models/`)

### What's next (Phase 3 ideas)

- Backlinks panel in the TUI sidebar
- Daily notes with automatic templates
- Vim keybinding mode for the editor
- Full-text search (FTS5) as complement to semantic search
- Auto-complete for wikilinks
- Export to HTML/PDF

---

## 2026-03-30 — Post-Phase-2 Bug Fixes & Registry Feature

### What was done

**Bug fixes:**

1. **ESC key broken in all TUI overlays** — Original root cause: bubbletea
   v1.3.10 maps the escape key to `"esc"`, not `"escape"`. Fixed string matching
   in 5 files, then upgraded all overlay key matching from string-based
   (`msg.String() == "esc"`) to type-based (`msg.Type == tea.KeyEscape`,
   `tea.KeyEnter`, `tea.KeyUp`, `tea.KeyDown`) for robustness. Also moved ESC
   handling for all overlays (palette, dialog, search, find bar) into `model.go`
   directly via `Hide()` pointer-receiver calls to avoid value-copy issues.
   **Status: still not working in Kitty terminal** despite `keydebug` tool
   confirming bubbletea receives `KeyEscape` correctly. Likely a Bubble Tea
   event routing or model value-semantics issue. Deferred to a future session.

2. **Scrolling large markdown files slow** — Root cause: `editor.Update()` was
   calling `m.textarea.Value()` (O(n) allocation — joins all lines) and
   `markdown.WordCount()` (O(n)) on every single message, including pure scroll
   and cursor-move events.  Fix:
   - Added `navigationKey()` helper to classify non-content-modifying keys.
   - `Value()` is now called only when a key event *could* change content; never
     for Up/Down/Left/Right/PgUp/PgDn/Home/End.
   - Added `lastContent string`, `lastLen int`, `modified bool` cache fields.
   - `Content()`, `Modified()`, `WordCount()` all return O(1) cached values.
   - `MarkSaved()` no longer calls `textarea.Value()`; uses `lastContent`.
   - Result: scroll events execute in O(1) rather than O(n).

3. **Embedding model not downloading / `--download-model` flag did nothing** —
   Two separate bugs:
   - `--download-model` flag in `bragi index` only printed a message but called
     the same `EnsureModel` which skips download if the cache exists.  Fixed: now
     `os.RemoveAll`s the model cache directory before downloading.
   - On TUI load, download failures were printed to stderr but swallowed behind
     the alt screen.  Fixed: `initSearch` now checks `ModelCached()` first,
     prints a clear message *before* the TUI launches, and streams byte-count
     progress to stderr.
   - Added `ModelCached(dir string) bool` helper to `embedding/download.go`.
   - Added `modelDownloadProgress()` goroutine helper in `tui/app.go` that
     renders progress percentages.

4. **ONNX model output name mismatch** — `NewONNXProvider` was hard-coding
   output names (`"token_embeddings"`, `"last_hidden_state"`), but the nomic
   model uses different names. `NewDynamicAdvancedSession` doesn't validate
   names at creation time — only at `Run()` time. Fixed by using
   `ort.GetInputOutputInfo(modelPath)` to auto-discover input/output tensor
   names from the ONNX file. Also:
   - Handles models with 2 inputs (no `token_type_ids`) or 3.
   - Handles output shapes of both `[batch, hidden]` (model-pooled) and
     `[batch, seq_len, hidden]` (needs mean pooling).
   - Supports models with multiple outputs (allocates correct number of slots).

**New feature — Global Project Registry:**

1. **`bragi open <name>` from anywhere** — New `internal/registry` package:
   - Registry file at `~/.config/bragi/registry.toml`
   - TOML format: `[projects]` table with `name = "absolute/path"` entries.
   - API: `Load()`, `LoadFrom(path)`, `Save()`, `Add()`, `AddOrUpdate()`,
     `Remove()`, `Resolve(name)`, `List()`, `ResolvePath(arg)`,
     `NameFromTitle(title)`.
   - `ResolvePath` tries exact name match → case-insensitive name match →
     fall through to filesystem path.
   - **`bragi new <title>`** auto-registers the created project (name derived
     from title via `NameFromTitle`; opt-out with `--no-register`).
   - **`bragi open <name-or-path>`** resolves name through registry first.
   - **`bragi list`** prints all registered projects, aligned.
   - **`bragi add <name> [path]`** manually registers an existing project.
   - **`bragi remove <name>`** removes a project from the registry.
   - 11 tests in `registry_test.go`, all passing.

### Stats

- 38 Go source files (was 34), 10 test files (was 9)
- 12 packages with code (was 11): added `registry`
- 45 passing tests (was 34)
- ~7,300 lines of Go (was ~6,200)

### Known issues

- **ESC key does not close overlays in Kitty terminal** — bubbletea receives
  `KeyEscape` correctly (verified with `cmd/keydebug` tool), but the palette /
  dialog / search / find bar do not close. Needs deeper investigation into
  Bubble Tea's model value-semantics or event routing.

### What's next (Phase 3 ideas)

- Fix ESC key issue
- Backlinks panel in the TUI sidebar
- Daily notes with automatic templates
- Vim keybinding mode for the editor
- Full-text search (FTS5) as complement to semantic search
- Auto-complete for wikilinks
- Export to HTML/PDF
