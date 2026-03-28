// Package config handles project configuration via project.toml.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// ProjectConfig is the root configuration struct, parsed from project.toml.
type ProjectConfig struct {
	Project       ProjectMeta         `toml:"project"`
	Editor        EditorConfig        `toml:"editor"`
	Search        SearchConfig        `toml:"search"`
	Wikilinks     WikilinkConfig      `toml:"wikilinks"`
	KnowledgeBase KnowledgeBaseConfig `toml:"knowledgebase"`
}

// ProjectMeta holds project identity and metadata.
type ProjectMeta struct {
	Title       string    `toml:"title"`
	Author      string    `toml:"author"`
	Description string    `toml:"description,omitempty"`
	Created     time.Time `toml:"created"`
	Methodology string    `toml:"methodology"` // para | zettelkasten | johnny-decimal | none
}

// EditorConfig holds Bragi-specific editor settings.
// Universal editor settings (tab size, indent style) belong in .editorconfig.
type EditorConfig struct {
	Theme                string `toml:"theme"`                      // dracula | alucard
	EditorMode           string `toml:"editor_mode"`                // standard | vim
	WordWrap             bool   `toml:"word_wrap"`
	SpellCheck           bool   `toml:"spell_check"`
	Autosave             bool   `toml:"autosave"`
	AutosaveIntervalSecs int    `toml:"autosave_interval_seconds"`
}

// SearchConfig holds semantic search and indexing settings.
type SearchConfig struct {
	EmbeddingModel     string   `toml:"embedding_model"`
	IndexOnSave        bool     `toml:"index_on_save"`
	ChunkStrategy      string   `toml:"chunk_strategy"`      // paragraph | heading
	ExcludeDirectories []string `toml:"exclude_directories"`
}

// WikilinkConfig holds wikilink engine behavior settings.
type WikilinkConfig struct {
	AutoComplete          bool `toml:"auto_complete"`
	ResolveAcrossSymlinks bool `toml:"resolve_across_symlinks"`
	ShowBacklinksPanel    bool `toml:"show_backlinks_panel"`
}

// KnowledgeBaseConfig holds methodology-specific settings.
type KnowledgeBaseConfig struct {
	EnableDailyNotes    bool                `toml:"enable_daily_notes"`
	DailyNotesDirectory string              `toml:"daily_notes_directory"`
	Zettelkasten        ZettelkastenConfig  `toml:"zettelkasten"`
	JohnnyDecimal       JohnnyDecimalConfig `toml:"johnny-decimal"`
}

// ZettelkastenConfig holds Zettelkasten-specific settings.
// Ignored if methodology != "zettelkasten".
type ZettelkastenConfig struct {
	IDFormat                string `toml:"id_format"`                   // timestamp | incremental
	InboxAutoReviewReminder bool   `toml:"inbox_auto_review_reminder"`
}

// JohnnyDecimalConfig holds Johnny Decimal-specific settings.
// Ignored if methodology != "johnny-decimal".
type JohnnyDecimalConfig struct {
	StrictNaming  bool `toml:"strict_naming"`
	AutoIncrement bool `toml:"auto_increment"`
}

// Valid methodology values.
var ValidMethodologies = []string{"para", "zettelkasten", "johnny-decimal", "none"}

// Valid theme values.
var ValidThemes = []string{"dracula", "alucard"}

// Valid editor modes.
var ValidEditorModes = []string{"standard", "vim"}

// Valid chunk strategies.
var ValidChunkStrategies = []string{"paragraph", "heading"}

// DefaultConfig returns a ProjectConfig with all defaults applied.
// A bare project.toml with only title, author, and methodology produces
// a working project using these defaults.
func DefaultConfig(title, author, methodology string) ProjectConfig {
	return ProjectConfig{
		Project: ProjectMeta{
			Title:       title,
			Author:      author,
			Created:     time.Now(),
			Methodology: methodology,
		},
		Editor: EditorConfig{
			Theme:                "dracula",
			EditorMode:           "standard",
			WordWrap:             true,
			SpellCheck:           false,
			Autosave:             true,
			AutosaveIntervalSecs: 30,
		},
		Search: SearchConfig{
			EmbeddingModel:     "nomic-ai/nomic-embed-text-v1.5",
			IndexOnSave:        true,
			ChunkStrategy:      "paragraph",
			ExcludeDirectories: []string{},
		},
		Wikilinks: WikilinkConfig{
			AutoComplete:          true,
			ResolveAcrossSymlinks: true,
			ShowBacklinksPanel:    true,
		},
		KnowledgeBase: KnowledgeBaseConfig{
			EnableDailyNotes:    true,
			DailyNotesDirectory: "journal",
			Zettelkasten: ZettelkastenConfig{
				IDFormat:                "timestamp",
				InboxAutoReviewReminder: true,
			},
			JohnnyDecimal: JohnnyDecimalConfig{
				StrictNaming:  true,
				AutoIncrement: true,
			},
		},
	}
}

// ConfigFileName is the expected project configuration filename.
const ConfigFileName = "project.toml"

// Load reads and parses a project.toml from the given directory.
func Load(dir string) (ProjectConfig, error) {
	path := filepath.Join(dir, ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return ProjectConfig{}, fmt.Errorf("reading %s: %w", path, err)
	}

	// Start with defaults so missing fields get sane values.
	cfg := DefaultConfig("", "", "none")
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return ProjectConfig{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return ProjectConfig{}, fmt.Errorf("validating %s: %w", path, err)
	}

	return cfg, nil
}

// Save writes the ProjectConfig as TOML to project.toml in the given directory.
func Save(dir string, cfg ProjectConfig) error {
	path := filepath.Join(dir, ConfigFileName)
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// Validate checks that all config values are within allowed ranges.
func (c *ProjectConfig) Validate() error {
	if c.Project.Title == "" {
		return fmt.Errorf("project.title is required")
	}
	if !isValidValue(c.Project.Methodology, ValidMethodologies) {
		return fmt.Errorf("project.methodology must be one of %v, got %q", ValidMethodologies, c.Project.Methodology)
	}
	if !isValidValue(c.Editor.Theme, ValidThemes) {
		return fmt.Errorf("editor.theme must be one of %v, got %q", ValidThemes, c.Editor.Theme)
	}
	if !isValidValue(c.Editor.EditorMode, ValidEditorModes) {
		return fmt.Errorf("editor.editor_mode must be one of %v, got %q", ValidEditorModes, c.Editor.EditorMode)
	}
	if !isValidValue(c.Search.ChunkStrategy, ValidChunkStrategies) {
		return fmt.Errorf("search.chunk_strategy must be one of %v, got %q", ValidChunkStrategies, c.Search.ChunkStrategy)
	}
	if c.Editor.AutosaveIntervalSecs < 1 {
		return fmt.Errorf("editor.autosave_interval_seconds must be >= 1, got %d", c.Editor.AutosaveIntervalSecs)
	}
	return nil
}

// DiscoverProjectDir walks up from startDir looking for a project.toml.
// Returns the directory containing it, or an error if none is found.
func DiscoverProjectDir(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ConfigFileName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s found in %s or any parent directory", ConfigFileName, startDir)
		}
		dir = parent
	}
}

func isValidValue(val string, allowed []string) bool {
	for _, a := range allowed {
		if val == a {
			return true
		}
	}
	return false
}
