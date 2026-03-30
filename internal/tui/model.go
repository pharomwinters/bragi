package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/adambick/bragi/internal/database"
	"github.com/adambick/bragi/internal/editor"
	"github.com/adambick/bragi/internal/filetree"
	"github.com/adambick/bragi/internal/knowledgebase"
	"github.com/adambick/bragi/internal/markdown"
	"github.com/adambick/bragi/internal/search"
	"github.com/adambick/bragi/internal/theme"
	"github.com/adambick/bragi/internal/wikilink"
)

const (
	sidebarMinWidth     = 25
	sidebarDefaultWidth = 30
)

// focus tracks which panel has keyboard focus.
type focus int

const (
	focusSidebar focus = iota
	focusEditor
)

// Model is the root Bubble Tea model for Bragi.
type Model struct {
	project   *knowledgebase.Project
	theme     theme.Theme
	parser    *markdown.Parser
	linkIndex *wikilink.PersistentIndex

	// Phase 2: search infrastructure (nil if unavailable).
	db        *database.DB
	indexer   *search.Indexer
	searchEng *search.Engine

	// UI components
	fileTree  filetree.Model
	editor    editor.Model
	statusBar StatusBar
	palette   Palette
	dialog    Dialog
	findBar   FindBar
	searchUI  Search
	commands  *CommandRegistry

	// Layout
	width        int
	height       int
	sidebarWidth int
	sidebarOpen  bool
	focus        focus

	// State
	lastError    string
	quitting     bool // true when we've asked about unsaved changes
}

// NewModel creates the root TUI model.
// The db, indexer, searchEng, and linkIdx parameters may be nil if search is unavailable.
func NewModel(
	proj *knowledgebase.Project,
	t theme.Theme,
	db *database.DB,
	indexer *search.Indexer,
	searchEng *search.Engine,
	linkIdx *wikilink.PersistentIndex,
) Model {
	cmds := NewCommandRegistry()

	var li *wikilink.PersistentIndex
	if linkIdx != nil {
		li = linkIdx
	}

	m := Model{
		project:      proj,
		theme:        t,
		parser:       markdown.NewParser(),
		linkIndex:    li,
		db:           db,
		indexer:      indexer,
		searchEng:    searchEng,
		commands:     cmds,
		sidebarWidth: sidebarDefaultWidth,
		sidebarOpen:  true,
		focus:        focusSidebar,
	}

	m.fileTree = filetree.New(t, sidebarDefaultWidth, 20)
	m.fileTree.SetFocused(true)
	m.editor = editor.New(t, 80, 20)
	m.statusBar = NewStatusBar(t, 80)
	m.palette = NewPalette(t, cmds, 80)
	m.dialog = NewDialog(t, 80)
	m.findBar = NewFindBar(t, 80)
	m.searchUI = NewSearch(t, searchEng, 80, 20)

	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.loadFiles(), m.editor.Init()}

	// Start polling the indexer status channel.
	if m.indexer != nil {
		cmds = append(cmds, m.pollIndexStatus())
	}

	return tea.Batch(cmds...)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Overlays (palette and dialog) get priority on input.
	if m.palette.Visible() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			// Handle ESC directly to avoid value-copy issues with palette.Update.
			if msg.Type == tea.KeyEscape {
				m.palette.Hide()
				m.statusBar.SetMessage("Palette closed (ESC)")
				return m, nil
			}
			var cmd tea.Cmd
			m.palette, cmd = m.palette.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		case paletteExecuteMsg:
			return m.executeCommand(msg.command)
		}
		// Non-key messages still flow through.
	}

	if m.dialog.Visible() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEscape {
				m.dialog.Hide()
				return m, nil
			}
			var cmd tea.Cmd
			m.dialog, cmd = m.dialog.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		case dialogConfirmMsg:
			return m.handleDialogConfirm(msg)
		}
	}

	if m.searchUI.Visible() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEscape {
				m.searchUI.Hide()
				return m, nil
			}
			var cmd tea.Cmd
			m.searchUI, cmd = m.searchUI.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		case searchDebounceMsg:
			if msg.query == m.searchUI.lastQuery && msg.time == m.searchUI.queryTime {
				return m, performSearch(m.searchEng, msg.query)
			}
			return m, nil
		case searchResultsMsg:
			if msg.err != nil {
				m.searchUI.loading = false
			} else {
				m.searchUI.results = msg.results
				m.searchUI.loading = false
				m.searchUI.cursor = 0
			}
			return m, nil
		case searchOpenResultMsg:
			return m, m.openFile(msg.relPath)
		}
	}

	if m.findBar.Visible() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEscape {
				m.findBar.Hide()
				m.focus = focusEditor
				m.editor.SetFocused(true)
				return m, nil
			}
			var cmd tea.Cmd
			m.findBar, cmd = m.findBar.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		case findQueryChangedMsg:
			if m.editor.Loaded() {
				matches := countMatches(m.editor.Content(), msg.query)
				current := 0
				if matches > 0 {
					current = 1
				}
				m.findBar.SetMatches(current, matches)
			}
		case findClosedMsg:
			// Return focus to editor.
			m.focus = focusEditor
			m.editor.SetFocused(true)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.editor.Modified() && !m.quitting {
				m.quitting = true
				m.dialog.ShowConfirm(dialogQuitUnsaved, "Unsaved Changes",
					"You have unsaved changes. Quit anyway?")
				return m, nil
			}
			return m, tea.Quit
		case "q":
			if m.focus == focusSidebar {
				if m.editor.Modified() && !m.quitting {
					m.quitting = true
					m.dialog.ShowConfirm(dialogQuitUnsaved, "Unsaved Changes",
						"You have unsaved changes. Quit anyway?")
					return m, nil
				}
				return m, tea.Quit
			}
		case "ctrl+f":
			if m.editor.Loaded() {
				m.findBar.Show()
				return m, nil
			}
		case "ctrl+p":
			m.palette.Show()
			return m, nil
		case "ctrl+k":
			m.searchUI.Show()
			return m, nil
		case "ctrl+n":
			m.dialog.ShowInput(dialogNewNote, "New Note", "Enter note title...")
			return m, nil
		case "tab":
			m.toggleFocus()
			return m, nil
		case "ctrl+b":
			m.sidebarOpen = !m.sidebarOpen
			m.updateLayout()
			return m, nil
		case "escape", "esc":
			if m.focus == focusEditor {
				m.toggleFocus()
				return m, nil
			}
		}

	case filetree.FileSelectedMsg:
		m.focus = focusEditor
		m.fileTree.SetFocused(false)
		m.editor.SetFocused(true)
		return m, m.openFile(msg.RelPath)

	case FileLoadedMsg:
		m.editor.LoadFile(msg.RelPath, msg.Content)
		m.lastError = ""

		doc := m.parser.Parse(msg.Content)
		links := wikilink.Extract(doc.Body)
		if m.linkIndex != nil {
			m.linkIndex.Update(msg.RelPath, links)
		}

		m.statusBar.SetFile(msg.RelPath, false)
		m.statusBar.SetWordCount(markdown.WordCount(doc.Body))

	case editor.SaveRequestMsg:
		return m, m.saveFile(msg.RelPath, msg.Content)

	case FileSavedMsg:
		m.editor.MarkSaved()
		m.statusBar.SetFile(msg.RelPath, false)
		m.statusBar.SetMessage("Saved")
		cmds = append(cmds, editor.ClearStatusAfter(2*time.Second))

		doc := m.parser.Parse(m.editor.Content())
		links := wikilink.Extract(doc.Body)
		if m.linkIndex != nil {
			m.linkIndex.Update(msg.RelPath, links)
		}

		// Enqueue for background reindexing.
		if m.indexer != nil && m.project.Config.Search.IndexOnSave {
			m.indexer.Enqueue(search.IndexRequest{
				RelPath: msg.RelPath,
				Content: m.editor.Content(),
			})
		}

	case editor.ClearStatusMsg:
		m.statusBar.ClearMessage()

	case ErrorMsg:
		m.lastError = msg.Err.Error()

	case StatusMsg:
		m.statusBar.SetMessage(msg.Text)

	case filesLoadedMsg:
		m.fileTree.SetFiles(msg.files)
		// If we have a persistent index, it already loaded from DB.
		// Otherwise, scan all files for wikilinks (Phase 1 fallback).
		if m.linkIndex == nil {
			// No persistent index; skip wikilink indexing.
		} else if m.db == nil {
			// No DB, so scan files for wikilinks.
			cmds = append(cmds, m.indexAllFiles(msg.files))
		}
		// If we have a DB, the wikilinks were already loaded by PersistentIndex.

	// Internal command messages.
	case paletteExecuteMsg:
		return m.executeCommand(msg.command)

	case dialogConfirmMsg:
		return m.handleDialogConfirm(msg)

	case showDialogMsg:
		switch msg.kind {
		case dialogNewNote:
			m.dialog.ShowInput(dialogNewNote, "New Note", "Enter note title...")
		case dialogRename:
			m.dialog.ShowInput(dialogRename, "Rename Note", "Enter new title...")
		case dialogDeleteConfirm:
			m.dialog.ShowConfirm(dialogDeleteConfirm, "Delete Note",
				fmt.Sprintf("Delete %q? This cannot be undone.", m.editor.RelPath()))
		}
		return m, nil

	case saveFromPaletteMsg:
		if m.editor.Loaded() {
			return m, m.saveFile(m.editor.RelPath(), m.editor.Content())
		}

	case toggleSidebarMsg:
		m.sidebarOpen = !m.sidebarOpen
		m.updateLayout()

	case openSearchMsg:
		m.searchUI.Show()
		return m, nil

	case reindexAllMsg:
		if m.indexer != nil {
			m.indexer.ReindexAll()
			m.statusBar.SetMessage("Reindexing all files...")
			cmds = append(cmds, editor.ClearStatusAfter(2*time.Second))
		} else {
			m.statusBar.SetMessage("Search not available")
			cmds = append(cmds, editor.ClearStatusAfter(2*time.Second))
		}

	case IndexProgressMsg:
		if msg.QueueSize > 0 {
			m.statusBar.SetMessage(fmt.Sprintf("Indexing... (%d remaining)", msg.QueueSize))
		} else if msg.QueueSize == 0 && msg.Done {
			m.statusBar.SetMessage("Indexing complete")
			cmds = append(cmds, editor.ClearStatusAfter(2*time.Second))
		}
		// QueueSize == -1 means timeout (no status), just keep polling.
		if m.indexer != nil {
			cmds = append(cmds, m.pollIndexStatus())
		}

	case switchThemeMsg:
		if m.theme.Name == "dracula" {
			m.theme = theme.Alucard
		} else {
			m.theme = theme.Dracula
		}
		// Propagate theme to all child components.
		m.editor.SetTheme(m.theme)
		m.fileTree.SetTheme(m.theme)
		m.statusBar.SetTheme(m.theme)
		m.palette.SetTheme(m.theme)
		m.dialog.SetTheme(m.theme)
		m.findBar.SetTheme(m.theme)
		m.searchUI.SetTheme(m.theme)
		m.statusBar.SetMessage("Theme: " + m.theme.Name)
		cmds = append(cmds, editor.ClearStatusAfter(2*time.Second))

	case noteCreatedMsg:
		m.statusBar.SetMessage("Created: " + msg.relPath)
		cmds = append(cmds, editor.ClearStatusAfter(2*time.Second))
		// Reload file tree and open the new note.
		cmds = append(cmds, m.loadFiles())
		cmds = append(cmds, m.openFile(msg.relPath))
		m.focus = focusEditor
		m.fileTree.SetFocused(false)
		m.editor.SetFocused(true)

	case noteRenamedMsg:
		if m.linkIndex != nil {
			m.linkIndex.Remove(msg.oldRel)
		}
		m.statusBar.SetMessage("Renamed to: " + msg.newRel)
		cmds = append(cmds, editor.ClearStatusAfter(2*time.Second))
		cmds = append(cmds, m.loadFiles())
		cmds = append(cmds, m.openFile(msg.newRel))

	case noteDeletedMsg:
		if m.linkIndex != nil {
			m.linkIndex.Remove(msg.relPath)
		}
		m.editor.LoadFile("", "")
		m.statusBar.SetFile("", false)
		m.statusBar.SetMessage("Deleted: " + msg.relPath)
		cmds = append(cmds, editor.ClearStatusAfter(2*time.Second))
		cmds = append(cmds, m.loadFiles())
		m.focus = focusSidebar
		m.fileTree.SetFocused(true)
		m.editor.SetFocused(false)
	}

	// Update status bar.
	if m.editor.Loaded() {
		m.statusBar.SetFile(m.editor.RelPath(), m.editor.Modified())
		m.statusBar.SetWordCount(m.editor.WordCount())
	}

	// Route updates to the focused component.
	switch m.focus {
	case focusSidebar:
		var cmd tea.Cmd
		m.fileTree, cmd = m.fileTree.Update(msg)
		cmds = append(cmds, cmd)
	case focusEditor:
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	contentHeight := m.height - 1

	var mainArea string
	if m.sidebarOpen {
		editorWidth := m.width - m.sidebarWidth
		if editorWidth < 10 {
			editorWidth = 10
		}

		sidebar := m.fileTree.View()
		editorView := m.renderEditorArea(editorWidth, contentHeight)

		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, editorView)
	} else {
		mainArea = m.renderEditorArea(m.width, contentHeight)
	}

	// Find bar sits between content and status bar.
	var findBarView string
	if m.findBar.Visible() {
		findBarView = m.findBar.View()
	}

	statusBar := m.statusBar.View()

	var view string
	if findBarView != "" {
		view = lipgloss.JoinVertical(lipgloss.Left, mainArea, findBarView, statusBar)
	} else {
		view = lipgloss.JoinVertical(lipgloss.Left, mainArea, statusBar)
	}

	// Render overlays on top.
	if m.palette.Visible() {
		overlay := m.palette.View()
		view = m.overlayCenter(view, overlay)
	}

	if m.dialog.Visible() {
		overlay := m.dialog.View()
		view = m.overlayCenter(view, overlay)
	}

	if m.searchUI.Visible() {
		overlay := m.searchUI.View()
		view = m.overlayCenter(view, overlay)
	}

	return view
}

// renderEditorArea renders the editor or error/empty state.
func (m Model) renderEditorArea(width, height int) string {
	if m.lastError != "" {
		style := lipgloss.NewStyle().
			Width(width).
			Height(height).
			Background(lipgloss.Color(string(m.theme.Background))).
			Padding(1, 2)
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(m.theme.Red)))
		return style.Render(errStyle.Render("Error: " + m.lastError))
	}

	return m.editor.View()
}

// overlayCenter places an overlay string in the upper-center of the base view.
func (m Model) overlayCenter(base, overlay string) string {
	// Simple approach: place the overlay at the top center.
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := lipgloss.Height(overlay)

	if overlayWidth >= m.width || overlayHeight >= m.height {
		return overlay
	}

	_ = base
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
}

// executeCommand runs a command palette action.
func (m Model) executeCommand(cmd Command) (tea.Model, tea.Cmd) {
	result := cmd.Action(&m)
	if result == nil {
		return m, nil
	}
	// Send the result as a message.
	return m, func() tea.Msg { return result }
}

// handleDialogConfirm processes dialog confirmations.
func (m Model) handleDialogConfirm(msg dialogConfirmMsg) (tea.Model, tea.Cmd) {
	switch msg.kind {
	case dialogNewNote:
		if msg.value == "" {
			return m, nil
		}
		return m, m.createNote(msg.value)

	case dialogRename:
		if msg.value == "" {
			return m, nil
		}
		return m, m.renameNote(m.editor.RelPath(), msg.value)

	case dialogDeleteConfirm:
		if msg.value == "yes" {
			return m, m.deleteNote(m.editor.RelPath())
		}

	case dialogQuitUnsaved:
		m.quitting = false
		if msg.value == "yes" {
			return m, tea.Quit
		}
	}

	return m, nil
}

// toggleFocus switches between sidebar and editor.
func (m *Model) toggleFocus() {
	switch m.focus {
	case focusSidebar:
		m.focus = focusEditor
		m.fileTree.SetFocused(false)
		m.editor.SetFocused(true)
	case focusEditor:
		m.focus = focusSidebar
		m.fileTree.SetFocused(true)
		m.editor.SetFocused(false)
	}
}

// updateLayout recalculates component sizes.
func (m *Model) updateLayout() {
	contentHeight := m.height - 1

	if m.sidebarOpen {
		sw := m.sidebarWidth
		if sw > m.width/2 {
			sw = m.width / 2
		}
		if sw < sidebarMinWidth {
			sw = sidebarMinWidth
		}
		m.sidebarWidth = sw
		m.fileTree.SetSize(sw, contentHeight)
		m.editor.SetSize(m.width-sw, contentHeight)
	} else {
		m.editor.SetSize(m.width, contentHeight)
	}

	m.statusBar.SetWidth(m.width)
	m.palette.SetWidth(m.width)
	m.dialog.SetWidth(m.width)
	m.findBar.SetWidth(m.width)
}

// loadFiles loads the project's file list into the sidebar.
func (m Model) loadFiles() tea.Cmd {
	return func() tea.Msg {
		files, err := m.project.ListMarkdownFiles()
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return filesLoadedMsg{files: files}
	}
}

type filesLoadedMsg struct {
	files []string
}

// openFile loads a file's content.
func (m Model) openFile(relPath string) tea.Cmd {
	return func() tea.Msg {
		content, err := m.project.ReadNote(relPath)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return FileLoadedMsg{RelPath: relPath, Content: content}
	}
}

// saveFile writes the editor content to disk.
func (m Model) saveFile(relPath, content string) tea.Cmd {
	return func() tea.Msg {
		if err := m.project.WriteNote(relPath, content); err != nil {
			return ErrorMsg{Err: err}
		}
		return FileSavedMsg{RelPath: relPath}
	}
}

// createNote creates a new note and refreshes the file tree.
func (m Model) createNote(title string) tea.Cmd {
	return func() tea.Msg {
		relPath, err := m.project.CreateNote(title, "")
		if err != nil {
			return ErrorMsg{Err: err}
		}
		// Return a sequence: reload files, then open the new note.
		return noteCreatedMsg{relPath: relPath}
	}
}

type noteCreatedMsg struct{ relPath string }

// renameNote renames the current note.
func (m Model) renameNote(oldRel, newTitle string) tea.Cmd {
	return func() tea.Msg {
		newRel, err := m.project.RenameNote(oldRel, newTitle)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return noteRenamedMsg{oldRel: oldRel, newRel: newRel}
	}
}

type noteRenamedMsg struct {
	oldRel string
	newRel string
}

// deleteNote deletes the current note.
func (m Model) deleteNote(relPath string) tea.Cmd {
	return func() tea.Msg {
		if err := m.project.DeleteNote(relPath); err != nil {
			return ErrorMsg{Err: err}
		}
		return noteDeletedMsg{relPath: relPath}
	}
}

type noteDeletedMsg struct{ relPath string }

// indexAllFiles scans all project files for wikilinks on startup (Phase 1 fallback).
func (m Model) indexAllFiles(files []string) tea.Cmd {
	return func() tea.Msg {
		if m.linkIndex == nil {
			return nil
		}
		for _, f := range files {
			content, err := m.project.ReadNote(f)
			if err != nil {
				continue
			}
			doc := m.parser.Parse(content)
			links := wikilink.Extract(doc.Body)
			m.linkIndex.Update(f, links)
		}

		fileCount, linkCount, targetCount := m.linkIndex.Stats()
		return StatusMsg{
			Text: fmt.Sprintf("Indexed %d files, %d links, %d targets", fileCount, linkCount, targetCount),
		}
	}
}

// pollIndexStatus reads from the indexer's status channel and converts to a tea.Msg.
func (m Model) pollIndexStatus() tea.Cmd {
	return func() tea.Msg {
		if m.indexer == nil {
			return nil
		}
		select {
		case status := <-m.indexer.Status():
			return IndexProgressMsg{
				RelPath:   status.RelPath,
				QueueSize: status.QueueSize,
				Done:      status.Done,
				Err:       status.Err,
			}
		case <-time.After(500 * time.Millisecond):
			// No status yet, poll again.
			return IndexProgressMsg{QueueSize: -1}
		}
	}
}
