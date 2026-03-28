package tui

// Command represents a command palette action.
type Command struct {
	Name        string
	Description string
	Action      func(m *Model) interface{} // returns a tea.Msg or nil
}

// CommandRegistry holds all available commands.
type CommandRegistry struct {
	commands []Command
}

// NewCommandRegistry creates the default command registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: []Command{
			{
				Name:        "New Note",
				Description: "Create a new markdown note",
				Action: func(m *Model) interface{} {
					return showDialogMsg{kind: dialogNewNote}
				},
			},
			{
				Name:        "Rename Note",
				Description: "Rename the current note",
				Action: func(m *Model) interface{} {
					if m.editor.RelPath() == "" {
						return StatusMsg{Text: "No file open to rename"}
					}
					return showDialogMsg{kind: dialogRename}
				},
			},
			{
				Name:        "Delete Note",
				Description: "Delete the current note",
				Action: func(m *Model) interface{} {
					if m.editor.RelPath() == "" {
						return StatusMsg{Text: "No file open to delete"}
					}
					return showDialogMsg{kind: dialogDeleteConfirm}
				},
			},
			{
				Name:        "Save",
				Description: "Save the current file",
				Action: func(m *Model) interface{} {
					if !m.editor.Loaded() {
						return StatusMsg{Text: "No file open"}
					}
					return saveFromPaletteMsg{}
				},
			},
			{
				Name:        "Toggle Sidebar",
				Description: "Show or hide the file sidebar",
				Action: func(m *Model) interface{} {
					return toggleSidebarMsg{}
				},
			},
			{
				Name:        "Switch Theme",
				Description: "Toggle between Dracula and Alucard",
				Action: func(m *Model) interface{} {
					return switchThemeMsg{}
				},
			},
		},
	}
}

// Commands returns all commands, optionally filtered by query.
func (r *CommandRegistry) Commands() []Command {
	return r.commands
}

// Internal messages for command actions.
type showDialogMsg struct{ kind dialogKind }
type saveFromPaletteMsg struct{}
type toggleSidebarMsg struct{}
type switchThemeMsg struct{}
