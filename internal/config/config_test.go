package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("Test KB", "Alice", "zettelkasten")

	if cfg.Project.Title != "Test KB" {
		t.Errorf("expected title 'Test KB', got %q", cfg.Project.Title)
	}
	if cfg.Project.Author != "Alice" {
		t.Errorf("expected author 'Alice', got %q", cfg.Project.Author)
	}
	if cfg.Project.Methodology != "zettelkasten" {
		t.Errorf("expected methodology 'zettelkasten', got %q", cfg.Project.Methodology)
	}
	if cfg.Editor.Theme != "dracula" {
		t.Errorf("expected default theme 'dracula', got %q", cfg.Editor.Theme)
	}
	if cfg.Editor.AutosaveIntervalSecs != 30 {
		t.Errorf("expected autosave interval 30, got %d", cfg.Editor.AutosaveIntervalSecs)
	}
	if cfg.KnowledgeBase.DailyNotesDirectory != "journal" {
		t.Errorf("expected daily notes dir 'journal', got %q", cfg.KnowledgeBase.DailyNotesDirectory)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig("Round Trip", "Bob", "para")

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Project.Title != "Round Trip" {
		t.Errorf("expected title 'Round Trip', got %q", loaded.Project.Title)
	}
	if loaded.Project.Methodology != "para" {
		t.Errorf("expected methodology 'para', got %q", loaded.Project.Methodology)
	}
	if loaded.Editor.Theme != "dracula" {
		t.Errorf("expected theme 'dracula', got %q", loaded.Editor.Theme)
	}
	if loaded.Editor.WordWrap != true {
		t.Errorf("expected word_wrap true")
	}
}

func TestLoadMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	// A minimal project.toml — only required fields. Defaults should fill the rest.
	minimal := `[project]
title = "Minimal"
author = "Eve"
methodology = "none"
created = 2026-03-27T00:00:00Z
`
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(minimal), 0644); err != nil {
		t.Fatalf("writing minimal config: %v", err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Project.Title != "Minimal" {
		t.Errorf("expected title 'Minimal', got %q", cfg.Project.Title)
	}
	// Defaults should be applied for unspecified fields.
	if cfg.Editor.Theme != "dracula" {
		t.Errorf("expected default theme 'dracula', got %q", cfg.Editor.Theme)
	}
	if cfg.Editor.AutosaveIntervalSecs != 30 {
		t.Errorf("expected default autosave interval 30, got %d", cfg.Editor.AutosaveIntervalSecs)
	}
	if cfg.Search.EmbeddingModel != "nomic-ai/nomic-embed-text-v1.5" {
		t.Errorf("expected default embedding model, got %q", cfg.Search.EmbeddingModel)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ProjectConfig)
		wantErr bool
	}{
		{
			name:    "valid config",
			mutate:  func(c *ProjectConfig) {},
			wantErr: false,
		},
		{
			name:    "empty title",
			mutate:  func(c *ProjectConfig) { c.Project.Title = "" },
			wantErr: true,
		},
		{
			name:    "invalid methodology",
			mutate:  func(c *ProjectConfig) { c.Project.Methodology = "bullet-journal" },
			wantErr: true,
		},
		{
			name:    "invalid theme",
			mutate:  func(c *ProjectConfig) { c.Editor.Theme = "monokai" },
			wantErr: true,
		},
		{
			name:    "invalid editor mode",
			mutate:  func(c *ProjectConfig) { c.Editor.EditorMode = "emacs" },
			wantErr: true,
		},
		{
			name:    "invalid chunk strategy",
			mutate:  func(c *ProjectConfig) { c.Search.ChunkStrategy = "sentence" },
			wantErr: true,
		},
		{
			name:    "zero autosave interval",
			mutate:  func(c *ProjectConfig) { c.Editor.AutosaveIntervalSecs = 0 },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig("Test", "Author", "none")
			tt.mutate(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDiscoverProjectDir(t *testing.T) {
	// Create a nested directory structure with project.toml at the root.
	root := t.TempDir()
	sub := filepath.Join(root, "notes", "deep")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("creating nested dirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte(`[project]
title = "Discover"
author = "Test"
methodology = "none"
created = 2026-01-01T00:00:00Z
`), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	found, err := DiscoverProjectDir(sub)
	if err != nil {
		t.Fatalf("DiscoverProjectDir failed: %v", err)
	}
	if found != root {
		t.Errorf("expected %q, got %q", root, found)
	}
}

func TestDiscoverProjectDirNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := DiscoverProjectDir(dir)
	if err == nil {
		t.Error("expected error when no project.toml exists")
	}
}
