// keydebug — tiny bubbletea program that prints raw key info.
// Run: go run ./cmd/keydebug/
// Press keys and see how bubbletea interprets them. Press Ctrl+C to quit.
package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	lines []string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		line := fmt.Sprintf("KeyMsg  Type=%-20v  String=%-20q  Runes=%v  Alt=%v",
			msg.Type, msg.String(), msg.Runes, msg.Alt)
		m.lines = append(m.lines, line)

		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		// Also check our specific concern:
		if msg.Type == tea.KeyEscape {
			m.lines = append(m.lines, "  >>> KeyEscape DETECTED <<<")
		}
		if msg.String() == "esc" {
			m.lines = append(m.lines, "  >>> String match 'esc' <<<")
		}

	case tea.MouseMsg:
		// Don't flood with mouse — just note it.
	default:
		m.lines = append(m.lines, fmt.Sprintf("Other: %T", msg))
	}

	// Keep last 20 lines.
	if len(m.lines) > 20 {
		m.lines = m.lines[len(m.lines)-20:]
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString("Press any key (Ctrl+C to quit). ESC key debug:\n\n")
	for _, l := range m.lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

func main() {
	p := tea.NewProgram(
		model{},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // same options as Bragi
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
