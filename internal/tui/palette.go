package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/adambick/bragi/internal/theme"
)

// Palette is the command palette overlay.
type Palette struct {
	input     textinput.Model
	commands  *CommandRegistry
	filtered  []Command
	cursor    int
	visible   bool
	theme     theme.Theme
	width     int
}

// NewPalette creates a command palette.
func NewPalette(t theme.Theme, cmds *CommandRegistry, width int) Palette {
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.CharLimit = 100

	return Palette{
		input:    ti,
		commands: cmds,
		filtered: cmds.Commands(),
		theme:    t,
		width:    width,
	}
}

// Show opens the palette.
func (p *Palette) Show() {
	p.visible = true
	p.input.SetValue("")
	p.input.Focus()
	p.cursor = 0
	p.filtered = p.commands.Commands()
}

// Hide closes the palette.
func (p *Palette) Hide() {
	p.visible = false
	p.input.Blur()
}

// Visible returns whether the palette is open.
func (p Palette) Visible() bool {
	return p.visible
}

// SetWidth sets the palette width.
func (p *Palette) SetWidth(w int) {
	p.width = w
}

// Update handles input for the palette.
func (p Palette) Update(msg tea.Msg) (Palette, tea.Cmd) {
	if !p.visible {
		return p, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "escape":
			p.Hide()
			return p, nil

		case "enter":
			if p.cursor < len(p.filtered) {
				cmd := p.filtered[p.cursor]
				p.Hide()
				// Return the command's action as a message.
				return p, func() tea.Msg {
					return paletteExecuteMsg{command: cmd}
				}
			}
			p.Hide()
			return p, nil

		case "up":
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil

		case "down":
			if p.cursor < len(p.filtered)-1 {
				p.cursor++
			}
			return p, nil
		}
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)

	// Filter commands based on input.
	query := strings.ToLower(p.input.Value())
	if query == "" {
		p.filtered = p.commands.Commands()
	} else {
		p.filtered = nil
		for _, c := range p.commands.Commands() {
			if strings.Contains(strings.ToLower(c.Name), query) ||
				strings.Contains(strings.ToLower(c.Description), query) {
				p.filtered = append(p.filtered, c)
			}
		}
	}

	// Clamp cursor.
	if p.cursor >= len(p.filtered) {
		p.cursor = len(p.filtered) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}

	return p, cmd
}

// View renders the palette overlay.
func (p Palette) View() string {
	if !p.visible {
		return ""
	}

	paletteWidth := p.width / 2
	if paletteWidth < 40 {
		paletteWidth = 40
	}
	if paletteWidth > 80 {
		paletteWidth = 80
	}

	boxStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(string(p.theme.BackgroundLight))).
		Foreground(lipgloss.Color(string(p.theme.Foreground))).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(string(p.theme.Purple))).
		Padding(0, 1).
		Width(paletteWidth)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(p.theme.Foreground))).
		Background(lipgloss.Color(string(p.theme.BackgroundLight)))

	var items []string
	for i, cmd := range p.filtered {
		nameStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(p.theme.Foreground)))
		descStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(p.theme.Comment)))

		if i == p.cursor {
			nameStyle = nameStyle.
				Foreground(lipgloss.Color(string(p.theme.Pink))).
				Bold(true)
			descStyle = descStyle.
				Foreground(lipgloss.Color(string(p.theme.Cyan)))
		}

		line := nameStyle.Render(cmd.Name) + "  " + descStyle.Render(cmd.Description)
		items = append(items, line)

		// Show max 8 items.
		if i >= 7 {
			break
		}
	}

	content := inputStyle.Render(p.input.View()) + "\n" + strings.Join(items, "\n")

	return boxStyle.Render(content)
}

// paletteExecuteMsg carries a command to execute.
type paletteExecuteMsg struct {
	command Command
}
