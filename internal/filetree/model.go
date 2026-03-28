// Package filetree implements the sidebar file tree component.
package filetree

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/adambick/bragi/internal/theme"
)

// FileSelectedMsg is emitted when a file is selected in the tree.
type FileSelectedMsg struct {
	RelPath string
}

// fileItem implements list.Item for the file tree.
type fileItem struct {
	relPath string
	name    string
	isDir   bool
	depth   int
}

func (f fileItem) Title() string {
	prefix := strings.Repeat("  ", f.depth)
	icon := "📄 "
	if f.isDir {
		icon = "📁 "
	}
	return prefix + icon + f.name
}

func (f fileItem) Description() string { return "" }
func (f fileItem) FilterValue() string { return f.relPath }

// Model is the Bubble Tea model for the file tree sidebar.
type Model struct {
	list     list.Model
	files    []fileItem
	width    int
	height   int
	theme    theme.Theme
	focused  bool
	selected string // currently selected file path
}

// New creates a new file tree model.
func New(t theme.Theme, width, height int) Model {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false

	// Style the delegate with theme colors.
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color(string(t.Pink))).
		BorderLeftForeground(lipgloss.Color(string(t.Pink)))
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(lipgloss.Color(string(t.Foreground)))

	l := list.New([]list.Item{}, delegate, width, height)
	l.Title = "Files"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()

	// Style the list title.
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(string(t.Purple))).
		Padding(0, 1)

	return Model{
		list:  l,
		theme: t,
		width: width,
		height: height,
	}
}

// SetFiles updates the file tree with a new list of relative paths.
func (m *Model) SetFiles(files []string) {
	// Build a tree-like flat list from relative paths.
	m.files = buildFileList(files)

	items := make([]list.Item, len(m.files))
	for i, f := range m.files {
		items[i] = f
	}
	m.list.SetItems(items)
}

// SetSize updates the component dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
}

// SetFocused sets whether this component has focus.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// Focused returns whether this component has focus.
func (m Model) Focused() bool {
	return m.focused
}

// Selected returns the currently selected file path.
func (m Model) Selected() string {
	return m.selected
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(fileItem); ok && !item.isDir {
				m.selected = item.relPath
				return m, func() tea.Msg {
					return FileSelectedMsg{RelPath: item.relPath}
				}
			}
		}
	}

	if m.focused {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m Model) View() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Background(lipgloss.Color(string(m.theme.BackgroundDark)))

	if m.focused {
		style = style.BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeftForeground(lipgloss.Color(string(m.theme.Purple)))
	}

	return style.Render(m.list.View())
}

// buildFileList converts relative paths into a flat file list with hierarchy info.
func buildFileList(files []string) []fileItem {
	// Track which directories we've already added.
	dirsSeen := make(map[string]bool)
	var items []fileItem

	// Sort files for consistent ordering.
	sort.Strings(files)

	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f), "/")

		// Add directory entries for parent paths.
		for i := 0; i < len(parts)-1; i++ {
			dirPath := strings.Join(parts[:i+1], "/")
			if !dirsSeen[dirPath] {
				dirsSeen[dirPath] = true
				items = append(items, fileItem{
					relPath: dirPath,
					name:    parts[i],
					isDir:   true,
					depth:   i,
				})
			}
		}

		// Add the file entry.
		items = append(items, fileItem{
			relPath: f,
			name:    parts[len(parts)-1],
			isDir:   false,
			depth:   len(parts) - 1,
		})
	}

	return items
}
