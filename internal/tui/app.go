package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/adambick/bragi/internal/knowledgebase"
	"github.com/adambick/bragi/internal/theme"
)

// Run launches the Bubble Tea TUI application.
func Run(proj *knowledgebase.Project) error {
	t := theme.ByName(proj.Config.Editor.Theme)
	m := NewModel(proj, t)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err
}
