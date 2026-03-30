// Package editor implements the markdown text editor component.
package editor

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/adambick/bragi/internal/markdown"
	"github.com/adambick/bragi/internal/theme"
)

// SaveRequestMsg is sent when the user presses Ctrl+S.
type SaveRequestMsg struct {
	RelPath string
	Content string
}

// ClearStatusMsg is sent after a delay to clear status messages.
type ClearStatusMsg struct{}

// Model is the Bubble Tea model for the markdown editor.
type Model struct {
	textarea textarea.Model
	theme    theme.Theme
	width    int
	height   int
	focused  bool

	// File state
	relPath  string
	original string // content at load time, for modified detection
	loaded   bool

	// Cached state — updated only when content actually changes to avoid
	// O(n) comparisons on every scroll / cursor-move event.
	wordCount   int
	modified    bool
	lastLen     int    // byte length of last known content
	lastContent string // full content snapshot, refreshed on length change
}

// New creates a new editor model.
func New(t theme.Theme, width, height int) Model {
	ta := textarea.New()
	ta.Placeholder = "Select a file from the sidebar..."
	ta.ShowLineNumbers = true
	ta.CharLimit = 0 // unlimited

	// Apply theme colors.
	ta.FocusedStyle.Base = ta.FocusedStyle.Base.
		Background(lipgloss.Color(string(t.Background)))
	ta.FocusedStyle.Text = ta.FocusedStyle.Text.
		Foreground(lipgloss.Color(string(t.Foreground)))
	ta.FocusedStyle.Placeholder = ta.FocusedStyle.Placeholder.
		Foreground(lipgloss.Color(string(t.Comment)))
	ta.FocusedStyle.CursorLine = ta.FocusedStyle.CursorLine.
		Background(lipgloss.Color(string(t.CurrentLine)))
	ta.FocusedStyle.LineNumber = ta.FocusedStyle.LineNumber.
		Foreground(lipgloss.Color(string(t.Comment)))

	ta.BlurredStyle = ta.FocusedStyle

	ta.SetWidth(width)
	ta.SetHeight(height)

	return Model{
		textarea: ta,
		theme:    t,
		width:    width,
		height:   height,
	}
}

// LoadFile sets the editor content from a file.
func (m *Model) LoadFile(relPath, content string) {
	m.relPath = relPath
	m.original = content
	m.loaded = true
	m.textarea.SetValue(content)
	m.textarea.CursorStart()
	m.wordCount = markdown.WordCount(content)
	m.modified = false
	m.lastLen = len(content)
	m.lastContent = content
}

// SetTheme updates the editor's theme colors.
func (m *Model) SetTheme(t theme.Theme) {
	m.theme = t
	m.textarea.FocusedStyle.Base = m.textarea.FocusedStyle.Base.
		Background(lipgloss.Color(string(t.Background)))
	m.textarea.FocusedStyle.Text = m.textarea.FocusedStyle.Text.
		Foreground(lipgloss.Color(string(t.Foreground)))
	m.textarea.FocusedStyle.Placeholder = m.textarea.FocusedStyle.Placeholder.
		Foreground(lipgloss.Color(string(t.Comment)))
	m.textarea.FocusedStyle.CursorLine = m.textarea.FocusedStyle.CursorLine.
		Background(lipgloss.Color(string(t.CurrentLine)))
	m.textarea.FocusedStyle.LineNumber = m.textarea.FocusedStyle.LineNumber.
		Foreground(lipgloss.Color(string(t.Comment)))
	m.textarea.BlurredStyle = m.textarea.FocusedStyle
}

// SetSize updates the editor dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.textarea.SetWidth(width)
	m.textarea.SetHeight(height)
}

// SetFocused sets whether the editor has focus.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
	if focused {
		m.textarea.Focus()
	} else {
		m.textarea.Blur()
	}
}

// Focused returns whether the editor is focused.
func (m Model) Focused() bool {
	return m.focused
}

// Content returns the current editor content.
// Uses the cached snapshot — O(1). Updated on every content-modifying event.
func (m Model) Content() string {
	return m.lastContent
}

// RelPath returns the currently loaded file path.
func (m Model) RelPath() string {
	return m.relPath
}

// Modified returns whether the content has been modified since load.
// Returns the cached state — O(1).
func (m Model) Modified() bool {
	if !m.loaded {
		return false
	}
	return m.modified
}

// WordCount returns the current word count.
func (m Model) WordCount() int {
	return m.wordCount
}

// Loaded returns whether a file is loaded.
func (m Model) Loaded() bool {
	return m.loaded
}

// MarkSaved updates the original content after a successful save.
func (m *Model) MarkSaved() {
	m.original = m.lastContent
	m.modified = false
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// navigationKey returns true for keys that move the cursor or scroll but
// cannot modify content. Value() is never called for these.
func navigationKey(s string) bool {
	switch s {
	case "up", "down", "left", "right",
		"pgup", "pgdown", "home", "end",
		"ctrl+up", "ctrl+down", "ctrl+left", "ctrl+right",
		"shift+up", "shift+down", "shift+left", "shift+right",
		"alt+left", "alt+right":
		return true
	}
	return false
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused || !m.loaded {
		return m, nil
	}

	var cmds []tea.Cmd

	// Determine whether this message can possibly change content before we
	// hand it to the textarea. Pure navigation and mouse events cannot.
	contentMayChange := true
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			// Request save using the cached content snapshot.
			return m, func() tea.Msg {
				return SaveRequestMsg{
					RelPath: m.relPath,
					Content: m.lastContent,
				}
			}
		default:
			contentMayChange = !navigationKey(msg.String())
		}
	case tea.MouseMsg:
		contentMayChange = false
	default:
		contentMayChange = false
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	// Only call Value() (O(n)) when the key event could have changed content.
	// Scroll / cursor events are handled in O(1).
	if contentMayChange {
		newVal := m.textarea.Value() // single allocation
		newLen := len(newVal)
		m.lastContent = newVal
		if newLen != m.lastLen {
			m.lastLen = newLen
			m.wordCount = markdown.WordCount(newVal)
		}
		m.modified = (newVal != m.original)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.loaded {
		style := lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Background(lipgloss.Color(string(m.theme.Background))).
			Foreground(lipgloss.Color(string(m.theme.Comment))).
			Padding(1, 2).
			Italic(true)

		hint := strings.Join([]string{
			"Select a file from the sidebar to begin editing.",
			"",
			"  Tab     — switch focus",
			"  Ctrl+B  — toggle sidebar",
			"  q       — quit (from sidebar)",
		}, "\n")

		return style.Render(hint)
	}

	return m.textarea.View()
}

// clearStatusAfter returns a command that sends ClearStatusMsg after a delay.
func ClearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return ClearStatusMsg{}
	})
}
