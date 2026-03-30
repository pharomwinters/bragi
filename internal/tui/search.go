package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/adambick/bragi/internal/search"
	"github.com/adambick/bragi/internal/theme"
)

const (
	searchDebounce   = 300 * time.Millisecond
	searchMaxResults = 10
)

// Search is the semantic search overlay component.
type Search struct {
	input   textinput.Model
	results []search.SearchResult
	cursor  int
	visible bool
	loading bool
	theme   theme.Theme
	width   int
	height  int
	engine  *search.Engine

	// Debounce state.
	lastQuery string
	queryTime time.Time
}

// NewSearch creates a search overlay.
func NewSearch(t theme.Theme, engine *search.Engine, width, height int) Search {
	ti := textinput.New()
	ti.Placeholder = "Search your knowledge base..."
	ti.CharLimit = 200

	return Search{
		input:  ti,
		engine: engine,
		theme:  t,
		width:  width,
		height: height,
	}
}

// SetTheme updates the search overlay's theme.
func (s *Search) SetTheme(t theme.Theme) {
	s.theme = t
}

// Show opens the search overlay.
func (s *Search) Show() {
	s.visible = true
	s.input.SetValue("")
	s.input.Focus()
	s.cursor = 0
	s.results = nil
	s.loading = false
	s.lastQuery = ""
}

// Hide closes the search overlay.
func (s *Search) Hide() {
	s.visible = false
	s.input.Blur()
}

// Visible returns whether the search overlay is open.
func (s Search) Visible() bool {
	return s.visible
}

// SetSize updates the overlay dimensions.
func (s *Search) SetSize(w, h int) {
	s.width = w
	s.height = h
}

// Update handles input for the search overlay.
func (s Search) Update(msg tea.Msg) (Search, tea.Cmd) {
	if !s.visible {
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEscape:
			s.Hide()
			return s, nil

		case msg.Type == tea.KeyEnter:
			if s.cursor < len(s.results) {
				result := s.results[s.cursor]
				s.Hide()
				return s, func() tea.Msg {
					return searchOpenResultMsg{
						relPath:   result.RelPath,
						startLine: result.StartLine,
					}
				}
			}
			return s, nil

		case msg.Type == tea.KeyUp:
			if s.cursor > 0 {
				s.cursor--
			}
			return s, nil

		case msg.Type == tea.KeyDown:
			if s.cursor < len(s.results)-1 {
				s.cursor++
			}
			return s, nil
		}
	}

	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)

	// Check if query changed and debounce.
	query := strings.TrimSpace(s.input.Value())
	if query != s.lastQuery && len(query) >= 2 {
		s.lastQuery = query
		s.queryTime = time.Now()
		s.loading = true

		// Return a debounced search command.
		capturedQuery := query
		capturedTime := s.queryTime
		return s, tea.Batch(cmd, func() tea.Msg {
			time.Sleep(searchDebounce)
			return searchDebounceMsg{
				query: capturedQuery,
				time:  capturedTime,
			}
		})
	}

	return s, cmd
}

// View renders the search overlay.
func (s Search) View() string {
	if !s.visible {
		return ""
	}

	overlayWidth := s.width * 3 / 5
	if overlayWidth < 50 {
		overlayWidth = 50
	}
	if overlayWidth > 100 {
		overlayWidth = 100
	}

	boxStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(string(s.theme.BackgroundLight))).
		Foreground(lipgloss.Color(string(s.theme.Foreground))).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(string(s.theme.Cyan))).
		Padding(0, 1).
		Width(overlayWidth)

	// Header.
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(s.theme.Cyan))).
		Bold(true)
	header := headerStyle.Render("Semantic Search")

	inputView := s.input.View()

	var body string
	if s.loading && len(s.results) == 0 {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(s.theme.Comment)))
		body = loadingStyle.Render("Searching...")
	} else if s.engine == nil {
		unavailStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(s.theme.Red)))
		body = unavailStyle.Render("Search unavailable: embedding model not loaded")
	} else if len(s.results) == 0 && s.lastQuery != "" && !s.loading {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(s.theme.Comment)))
		body = emptyStyle.Render("No results found")
	} else {
		var items []string
		maxItems := 8
		if maxItems > len(s.results) {
			maxItems = len(s.results)
		}

		for i := 0; i < maxItems; i++ {
			r := s.results[i]
			item := s.renderResult(r, i == s.cursor, overlayWidth-4)
			items = append(items, item)
		}
		body = strings.Join(items, "\n")
	}

	content := header + "\n" + inputView + "\n" + body

	return boxStyle.Render(content)
}

// renderResult renders a single search result.
func (s Search) renderResult(r search.SearchResult, selected bool, maxWidth int) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(s.theme.Foreground)))
	headingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(s.theme.Comment)))
	scoreStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(s.theme.Green)))
	snippetStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(s.theme.Comment)))

	if selected {
		titleStyle = titleStyle.
			Foreground(lipgloss.Color(string(s.theme.Pink))).
			Bold(true)
		headingStyle = headingStyle.
			Foreground(lipgloss.Color(string(s.theme.Cyan)))
		scoreStyle = scoreStyle.
			Foreground(lipgloss.Color(string(s.theme.Yellow)))
	}

	// Title line with score.
	title := r.Title
	if title == "" {
		title = r.RelPath
	}
	scoreBadge := scoreStyle.Render(fmt.Sprintf("%.0f%%", r.Score*100))
	titleLine := titleStyle.Render(title) + " " + scoreBadge

	// Heading context.
	var headingLine string
	if r.Heading != "" {
		headingLine = headingStyle.Render("  " + r.Heading)
	}

	// Snippet (truncated).
	snippet := strings.TrimSpace(r.ChunkText)
	if len(snippet) > maxWidth-4 {
		snippet = snippet[:maxWidth-7] + "..."
	}
	snippetLine := snippetStyle.Render("  " + snippet)

	if headingLine != "" {
		return titleLine + "\n" + headingLine + "\n" + snippetLine
	}
	return titleLine + "\n" + snippetLine
}

// Messages for the search overlay.

// searchDebounceMsg triggers a search after the debounce period.
type searchDebounceMsg struct {
	query string
	time  time.Time
}

// searchResultsMsg carries search results back to the model.
type searchResultsMsg struct {
	query   string
	results []search.SearchResult
	err     error
}

// searchOpenResultMsg requests opening a file from search results.
type searchOpenResultMsg struct {
	relPath   string
	startLine int
}

// performSearch runs the actual search and returns results.
func performSearch(engine *search.Engine, query string) tea.Cmd {
	return func() tea.Msg {
		if engine == nil {
			return searchResultsMsg{query: query, err: fmt.Errorf("search engine not available")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		results, err := engine.Search(ctx, query, searchMaxResults)
		return searchResultsMsg{
			query:   query,
			results: results,
			err:     err,
		}
	}
}
