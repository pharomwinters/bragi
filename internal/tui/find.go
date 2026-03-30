package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/adambick/bragi/internal/theme"
)

// FindBar is an inline find-in-file component (Ctrl+F).
type FindBar struct {
	input    textinput.Model
	visible  bool
	theme    theme.Theme
	width    int
	query    string
	matches  int
	current  int // 1-based index of the current match
}

// NewFindBar creates a find bar.
func NewFindBar(t theme.Theme, width int) FindBar {
	ti := textinput.New()
	ti.Placeholder = "Find..."
	ti.CharLimit = 200

	return FindBar{
		input: ti,
		theme: t,
		width: width,
	}
}

// SetTheme updates the find bar's theme.
func (f *FindBar) SetTheme(t theme.Theme) {
	f.theme = t
}

// Show opens the find bar.
func (f *FindBar) Show() {
	f.visible = true
	f.input.SetValue("")
	f.input.Focus()
	f.query = ""
	f.matches = 0
	f.current = 0
}

// Hide closes the find bar.
func (f *FindBar) Hide() {
	f.visible = false
	f.input.Blur()
}

// Visible returns whether the find bar is shown.
func (f FindBar) Visible() bool {
	return f.visible
}

// Query returns the current search query.
func (f FindBar) Query() string {
	return f.query
}

// SetWidth updates the find bar width.
func (f *FindBar) SetWidth(w int) {
	f.width = w
}

// SetMatches updates the match count display.
func (f *FindBar) SetMatches(current, total int) {
	f.current = current
	f.matches = total
}

// Update handles find bar input.
func (f FindBar) Update(msg tea.Msg) (FindBar, tea.Cmd) {
	if !f.visible {
		return f, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEscape:
			f.Hide()
			return f, func() tea.Msg {
				return findClosedMsg{}
			}

		case msg.Type == tea.KeyEnter:
			// Find next match.
			return f, func() tea.Msg {
				return findNextMsg{query: f.query}
			}

		case msg.String() == "shift+enter":
			// Find previous match.
			return f, func() tea.Msg {
				return findPrevMsg{query: f.query}
			}
		}
	}

	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)

	// Update query when input changes.
	newQuery := f.input.Value()
	if newQuery != f.query {
		f.query = newQuery
		return f, tea.Batch(cmd, func() tea.Msg {
			return findQueryChangedMsg{query: newQuery}
		})
	}

	return f, cmd
}

// View renders the find bar.
func (f FindBar) View() string {
	if !f.visible {
		return ""
	}

	style := lipgloss.NewStyle().
		Background(lipgloss.Color(string(f.theme.BackgroundLight))).
		Foreground(lipgloss.Color(string(f.theme.Foreground))).
		Padding(0, 1).
		Width(f.width)

	inputView := f.input.View()

	var matchInfo string
	if f.query != "" {
		infoStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(f.theme.Comment)))
		if f.matches == 0 {
			matchInfo = infoStyle.Render(" No matches")
		} else {
			matchInfo = infoStyle.Render(fmt.Sprintf(" %d/%d", f.current, f.matches))
		}
	}

	return style.Render(inputView + matchInfo)
}

// countMatches counts occurrences of query in content (case-insensitive).
func countMatches(content, query string) int {
	if query == "" {
		return 0
	}
	return strings.Count(strings.ToLower(content), strings.ToLower(query))
}

// Find bar messages.
type findQueryChangedMsg struct{ query string }
type findNextMsg struct{ query string }
type findPrevMsg struct{ query string }
type findClosedMsg struct{}
