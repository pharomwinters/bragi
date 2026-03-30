package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/adambick/bragi/internal/theme"
)

// dialogKind identifies the type of dialog.
type dialogKind int

const (
	dialogNone dialogKind = iota
	dialogNewNote
	dialogRename
	dialogDeleteConfirm
	dialogQuitUnsaved
)

// Dialog is a modal dialog for user input or confirmation.
type Dialog struct {
	kind    dialogKind
	input   textinput.Model
	title   string
	message string
	visible bool
	theme   theme.Theme
	width   int
}

// NewDialog creates a dialog component.
func NewDialog(t theme.Theme, width int) Dialog {
	ti := textinput.New()
	ti.CharLimit = 200

	return Dialog{
		input: ti,
		theme: t,
		width: width,
	}
}

// SetTheme updates the dialog's theme.
func (d *Dialog) SetTheme(t theme.Theme) {
	d.theme = t
}

// ShowInput shows a text input dialog.
func (d *Dialog) ShowInput(kind dialogKind, title, placeholder string) {
	d.kind = kind
	d.title = title
	d.message = ""
	d.visible = true
	d.input.Placeholder = placeholder
	d.input.SetValue("")
	d.input.Focus()
}

// ShowConfirm shows a confirmation dialog.
func (d *Dialog) ShowConfirm(kind dialogKind, title, message string) {
	d.kind = kind
	d.title = title
	d.message = message
	d.visible = true
	d.input.Blur()
}

// Hide closes the dialog.
func (d *Dialog) Hide() {
	d.visible = false
	d.kind = dialogNone
	d.input.Blur()
}

// Visible returns whether the dialog is shown.
func (d Dialog) Visible() bool {
	return d.visible
}

// Kind returns the dialog type.
func (d Dialog) Kind() dialogKind {
	return d.kind
}

// Value returns the input text.
func (d Dialog) Value() string {
	return d.input.Value()
}

// SetWidth sets the dialog width.
func (d *Dialog) SetWidth(w int) {
	d.width = w
}

// Update handles dialog input.
func (d Dialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	if !d.visible {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEscape:
			d.Hide()
			return d, nil

		case msg.Type == tea.KeyEnter:
			kind := d.kind
			value := d.input.Value()
			d.Hide()
			return d, func() tea.Msg {
				return dialogConfirmMsg{kind: kind, value: value}
			}

		case msg.String() == "y" || msg.String() == "Y":
			if d.message != "" { // confirmation dialog
				kind := d.kind
				d.Hide()
				return d, func() tea.Msg {
					return dialogConfirmMsg{kind: kind, value: "yes"}
				}
			}

		case msg.String() == "n" || msg.String() == "N":
			if d.message != "" { // confirmation dialog
				d.Hide()
				return d, nil
			}
		}
	}

	// Update text input if this is an input dialog.
	if d.message == "" {
		var cmd tea.Cmd
		d.input, cmd = d.input.Update(msg)
		return d, cmd
	}

	return d, nil
}

// View renders the dialog overlay.
func (d Dialog) View() string {
	if !d.visible {
		return ""
	}

	dialogWidth := d.width / 3
	if dialogWidth < 40 {
		dialogWidth = 40
	}
	if dialogWidth > 60 {
		dialogWidth = 60
	}

	boxStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(string(d.theme.BackgroundLight))).
		Foreground(lipgloss.Color(string(d.theme.Foreground))).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(string(d.theme.Purple))).
		Padding(1, 2).
		Width(dialogWidth)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(string(d.theme.Purple)))

	content := titleStyle.Render(d.title) + "\n\n"

	if d.message != "" {
		// Confirmation dialog.
		content += d.message + "\n\n"
		hintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(d.theme.Comment)))
		content += hintStyle.Render("Press Y to confirm, N or Esc to cancel")
	} else {
		// Input dialog.
		content += d.input.View()
	}

	return boxStyle.Render(content)
}

// dialogConfirmMsg is sent when the user confirms a dialog.
type dialogConfirmMsg struct {
	kind  dialogKind
	value string
}
