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
