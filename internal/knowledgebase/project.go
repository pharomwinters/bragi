// Package knowledgebase handles project creation, loading, and file operations.
package knowledgebase

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adambick/bragi/internal/config"
)

// Project represents a loaded Bragi knowledge base project.
type Project struct {
	RootDir string
	Config  config.ProjectConfig
}

// methodologyDirs maps each methodology to its required subdirectories.
var methodologyDirs = map[string][]string{
	"para":           {"1-projects", "2-areas", "3-resources", "4-archives"},
	"zettelkasten":   {"inbox", "permanent", "literature", "maps"},
	"johnny-decimal": {},
	"none":           {},
}

// NewProject creates a new project directory with project.toml and
// methodology-specific subdirectories.
func NewProject(parentDir, title, author, methodology string) (*Project, error) {
	// Sanitize title into a directory name.
	dirName := sanitizeDirName(title)
	rootDir := filepath.Join(parentDir, dirName)

	// Don't overwrite an existing directory.
	if _, err := os.Stat(rootDir); err == nil {
		return nil, fmt.Errorf("directory already exists: %s", rootDir)
	}

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("creating project directory: %w", err)
	}

	cfg := config.DefaultConfig(title, author, methodology)

	// Create methodology-specific directories.
	dirs, ok := methodologyDirs[methodology]
	if !ok {
		return nil, fmt.Errorf("unknown methodology: %q", methodology)
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(rootDir, d), 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", d, err)
		}
	}

	// Create journal directory if daily notes are enabled.
	if cfg.KnowledgeBase.EnableDailyNotes {
		journalDir := filepath.Join(rootDir, cfg.KnowledgeBase.DailyNotesDirectory)
		if err := os.MkdirAll(journalDir, 0755); err != nil {
			return nil, fmt.Errorf("creating journal directory: %w", err)
		}
	}

	// Write project.toml.
	if err := config.Save(rootDir, cfg); err != nil {
		return nil, fmt.Errorf("writing project config: %w", err)
	}

	return &Project{RootDir: rootDir, Config: cfg}, nil
}

// OpenProject loads an existing project from the given directory.
// If the directory itself doesn't contain project.toml, it walks up
// the directory tree to find one.
func OpenProject(dir string) (*Project, error) {
	rootDir, err := config.DiscoverProjectDir(dir)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(rootDir)
	if err != nil {
		return nil, err
	}

	return &Project{RootDir: rootDir, Config: cfg}, nil
}

// ListMarkdownFiles returns all .md files in the project directory,
// with paths relative to the project root. Sorted alphabetically.
func (p *Project) ListMarkdownFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(p.RootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden directories and the database file.
		name := info.Name()
		if info.IsDir() && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(name), ".md") {
			rel, err := filepath.Rel(p.RootDir, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

// sanitizeDirName converts a project title into a filesystem-safe directory name.
func sanitizeDirName(title string) string {
	name := strings.ToLower(title)
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, name)
	// Collapse multiple dashes.
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	return strings.Trim(name, "-")
}
