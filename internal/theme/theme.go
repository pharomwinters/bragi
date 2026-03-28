// Package theme defines the Dracula and Alucard color themes for Bragi.
//
// Based on the Dracula Theme specification (https://draculatheme.com/spec).
// Created by Zeno Rocha, maintained by Zeno Rocha and Lucas de França.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines a complete color palette for the Bragi UI.
type Theme struct {
	Name string
	Dark bool

	// Core palette
	Background      lipgloss.Color
	BackgroundDark  lipgloss.Color  // sidebar
	BackgroundDarker lipgloss.Color // status bar
	BackgroundLight lipgloss.Color  // floating panels, command palette
	BackgroundLighter lipgloss.Color // hover states, subtle borders
	Foreground      lipgloss.Color
	CurrentLine     lipgloss.Color
	Selection       lipgloss.Color
	Comment         lipgloss.Color

	// Accent colors
	Red    lipgloss.Color
	Orange lipgloss.Color
	Yellow lipgloss.Color
	Green  lipgloss.Color
	Cyan   lipgloss.Color
	Purple lipgloss.Color
	Pink   lipgloss.Color
}

// FunctionalColors are shared across both themes for semantic UI states.
var FunctionalColors = struct {
	Red    lipgloss.Color // critical actions, error dialogs
	Orange lipgloss.Color // warnings
	Green  lipgloss.Color // success states
	Cyan   lipgloss.Color // information, links, help text
	Purple lipgloss.Color // focus indicators, active input borders
}{
	Red:    lipgloss.Color("#DE5735"),
	Orange: lipgloss.Color("#A39514"),
	Green:  lipgloss.Color("#089108"),
	Cyan:   lipgloss.Color("#0081D6"),
	Purple: lipgloss.Color("#815CD6"),
}

// Dracula is the default dark theme.
var Dracula = Theme{
	Name:              "dracula",
	Dark:              true,
	Background:        lipgloss.Color("#282A36"),
	BackgroundDark:    lipgloss.Color("#21222C"),
	BackgroundDarker:  lipgloss.Color("#191A21"),
	BackgroundLight:   lipgloss.Color("#343746"),
	BackgroundLighter: lipgloss.Color("#424450"),
	Foreground:        lipgloss.Color("#F8F8F2"),
	CurrentLine:       lipgloss.Color("#44475A"),
	Selection:         lipgloss.Color("#44475A"),
	Comment:           lipgloss.Color("#6272A4"),
	Red:               lipgloss.Color("#FF5555"),
	Orange:            lipgloss.Color("#FFB86C"),
	Yellow:            lipgloss.Color("#F1FA8C"),
	Green:             lipgloss.Color("#50FA7B"),
	Cyan:              lipgloss.Color("#8BE9FD"),
	Purple:            lipgloss.Color("#BD93F9"),
	Pink:              lipgloss.Color("#FF79C6"),
}

// Alucard is the light theme variant.
var Alucard = Theme{
	Name:              "alucard",
	Dark:              false,
	Background:        lipgloss.Color("#FFFBEB"),
	BackgroundDark:    lipgloss.Color("#CECCC0"),
	BackgroundDarker:  lipgloss.Color("#BCBAB3"),
	BackgroundLight:   lipgloss.Color("#EFEDDC"),
	BackgroundLighter: lipgloss.Color("#ECE9DF"),
	Foreground:        lipgloss.Color("#1F1F1F"),
	CurrentLine:       lipgloss.Color("#DEDCCF"),
	Selection:         lipgloss.Color("#CFCFDE"),
	Comment:           lipgloss.Color("#6C664B"),
	Red:               lipgloss.Color("#CB3A2A"),
	Orange:            lipgloss.Color("#A34D14"),
	Yellow:            lipgloss.Color("#846E15"),
	Green:             lipgloss.Color("#14710A"),
	Cyan:              lipgloss.Color("#036A96"),
	Purple:            lipgloss.Color("#644AC9"),
	Pink:              lipgloss.Color("#A3144D"),
}

// ByName returns the theme matching the given name.
// Defaults to Dracula if the name is unrecognized.
func ByName(name string) Theme {
	switch name {
	case "alucard":
		return Alucard
	default:
		return Dracula
	}
}
