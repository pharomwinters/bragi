package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/adambick/bragi/internal/theme"
)

// StatusBar renders the bottom status bar.
type StatusBar struct {
	theme    theme.Theme
	width    int
	fileName string
	modified bool
	words    int
	message  string // temporary status message
}

// NewStatusBar creates a status bar.
func NewStatusBar(t theme.Theme, width int) StatusBar {
	return StatusBar{
		theme: t,
		width: width,
	}
}

// SetWidth updates the status bar width.
func (s *StatusBar) SetWidth(w int) {
	s.width = w
}

// SetTheme updates the status bar's theme.
func (s *StatusBar) SetTheme(t theme.Theme) {
	s.theme = t
}

// SetFile updates the displayed filename and modified state.
func (s *StatusBar) SetFile(name string, modified bool) {
	s.fileName = name
	s.modified = modified
}

// SetWordCount updates the word count display.
func (s *StatusBar) SetWordCount(n int) {
	s.words = n
}

// SetMessage sets a temporary status message (e.g., "Saved").
func (s *StatusBar) SetMessage(msg string) {
	s.message = msg
}

// ClearMessage clears the temporary status message.
func (s *StatusBar) ClearMessage() {
	s.message = ""
}

// View renders the status bar.
func (s StatusBar) View() string {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color(string(s.theme.BackgroundDarker))).
		Foreground(lipgloss.Color(string(s.theme.Foreground))).
		Width(s.width).
		Padding(0, 1)

	// Left side: filename + modified indicator.
	left := s.fileName
	if left == "" {
		left = "No file open"
	}
	if s.modified {
		modStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(s.theme.Red))).
			Background(lipgloss.Color(string(s.theme.BackgroundDarker)))
		left += modStyle.Render(" ●")
	}

	// Right side: word count or status message.
	var right string
	if s.message != "" {
		msgStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(s.theme.Green))).
			Background(lipgloss.Color(string(s.theme.BackgroundDarker)))
		right = msgStyle.Render(s.message)
	} else if s.words > 0 {
		wcStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(s.theme.Orange))).
			Background(lipgloss.Color(string(s.theme.BackgroundDarker)))
		right = wcStyle.Render(fmt.Sprintf("%d words", s.words))
	}

	// Fill the gap between left and right.
	gap := s.width - lipgloss.Width(left) - lipgloss.Width(right) - 2 // padding
	if gap < 1 {
		gap = 1
	}
	fill := lipgloss.NewStyle().
		Background(lipgloss.Color(string(s.theme.BackgroundDarker))).
		Width(gap).
		Render("")

	return style.Render(left + fill + right)
}
