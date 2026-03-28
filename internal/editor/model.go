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

	// Word count
	wordCount int
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
func (m Model) Content() string {
	return m.textarea.Value()
}

// RelPath returns the currently loaded file path.
func (m Model) RelPath() string {
	return m.relPath
}

// Modified returns whether the content has been modified since load.
func (m Model) Modified() bool {
	if !m.loaded {
		return false
	}
	return m.textarea.Value() != m.original
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
	m.original = m.textarea.Value()
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused || !m.loaded {
		return m, nil
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			// Request save.
			return m, func() tea.Msg {
				return SaveRequestMsg{
					RelPath: m.relPath,
					Content: m.textarea.Value(),
				}
			}
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	// Update word count after any text change.
	m.wordCount = markdown.WordCount(m.textarea.Value())

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
