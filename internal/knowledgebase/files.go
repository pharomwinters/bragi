package knowledgebase

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CreateNote creates a new markdown file in the project.
// If dir is empty, the file is created at the project root.
// Returns the relative path of the created note.
func (p *Project) CreateNote(title, dir string) (string, error) {
	filename := sanitizeFileName(title) + ".md"

	var relPath string
	if dir != "" {
		relPath = filepath.Join(dir, filename)
	} else {
		relPath = filename
	}

	absPath := filepath.Join(p.RootDir, relPath)

	// Don't overwrite existing files.
	if _, err := os.Stat(absPath); err == nil {
		return "", fmt.Errorf("file already exists: %s", relPath)
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("creating parent directory: %w", err)
	}

	// Create the file with frontmatter.
	content := defaultFrontmatter(title)
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return relPath, nil
}

// RenameNote renames a note file. Both oldRel and newName are relative
// to the project root. newName is just the desired title — the .md
// extension and directory are preserved.
func (p *Project) RenameNote(oldRel, newTitle string) (string, error) {
	oldAbs := filepath.Join(p.RootDir, oldRel)
	if _, err := os.Stat(oldAbs); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", oldRel)
	}

	dir := filepath.Dir(oldRel)
	newFilename := sanitizeFileName(newTitle) + ".md"
	newRel := filepath.Join(dir, newFilename)
	newAbs := filepath.Join(p.RootDir, newRel)

	if _, err := os.Stat(newAbs); err == nil {
		return "", fmt.Errorf("target already exists: %s", newRel)
	}

	if err := os.Rename(oldAbs, newAbs); err != nil {
		return "", fmt.Errorf("renaming file: %w", err)
	}

	return newRel, nil
}

// DeleteNote removes a note file from the project.
func (p *Project) DeleteNote(relPath string) error {
	absPath := filepath.Join(p.RootDir, relPath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", relPath)
	}
	return os.Remove(absPath)
}

// ReadNote reads the contents of a note file.
func (p *Project) ReadNote(relPath string) (string, error) {
	absPath := filepath.Join(p.RootDir, relPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", relPath, err)
	}
	return string(data), nil
}

// WriteNote writes content to a note file.
func (p *Project) WriteNote(relPath, content string) error {
	absPath := filepath.Join(p.RootDir, relPath)
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", relPath, err)
	}
	return nil
}

// defaultFrontmatter generates YAML frontmatter for a new note.
func defaultFrontmatter(title string) string {
	return fmt.Sprintf(`---
title: %q
created: %s
---

`, title, time.Now().Format("2006-01-02"))
}

// sanitizeFileName converts a title to a filename-safe string (without extension).
func sanitizeFileName(title string) string {
	name := strings.ToLower(title)
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, name)
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	return strings.Trim(name, "-")
}
