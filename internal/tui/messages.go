// Package tui implements the Bubble Tea TUI application.
package tui

// FileSelectedMsg is sent when the user selects a file in the sidebar.
type FileSelectedMsg struct {
	RelPath string
}

// FileLoadedMsg is sent when a file's content has been loaded.
type FileLoadedMsg struct {
	RelPath string
	Content string
}

// FileSavedMsg is sent when a file has been saved.
type FileSavedMsg struct {
	RelPath string
}

// ErrorMsg reports an error to the UI.
type ErrorMsg struct {
	Err error
}

// StatusMsg displays a temporary message in the status bar.
type StatusMsg struct {
	Text string
}
