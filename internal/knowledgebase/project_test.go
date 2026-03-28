package knowledgebase

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewProject(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		methodology string
		wantDirs    []string
	}{
		{
			name:        "zettelkasten",
			title:       "My Zettelkasten",
			methodology: "zettelkasten",
			wantDirs:    []string{"inbox", "permanent", "literature", "maps", "journal"},
		},
		{
			name:        "para",
			title:       "My PARA",
			methodology: "para",
			wantDirs:    []string{"1-projects", "2-areas", "3-resources", "4-archives", "journal"},
		},
		{
			name:        "none",
			title:       "Plain Project",
			methodology: "none",
			wantDirs:    []string{"journal"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := t.TempDir()
			proj, err := NewProject(parent, tt.title, "Test Author", tt.methodology)
			if err != nil {
				t.Fatalf("NewProject failed: %v", err)
			}

			// Verify project.toml exists.
			if _, err := os.Stat(filepath.Join(proj.RootDir, "project.toml")); err != nil {
				t.Error("project.toml not found")
			}

			// Verify methodology directories exist.
			for _, d := range tt.wantDirs {
				if _, err := os.Stat(filepath.Join(proj.RootDir, d)); err != nil {
					t.Errorf("expected directory %q to exist", d)
				}
			}

			// Verify config was loaded correctly.
			if proj.Config.Project.Title != tt.title {
				t.Errorf("expected title %q, got %q", tt.title, proj.Config.Project.Title)
			}
			if proj.Config.Project.Methodology != tt.methodology {
				t.Errorf("expected methodology %q, got %q", tt.methodology, proj.Config.Project.Methodology)
			}
		})
	}
}

func TestNewProjectAlreadyExists(t *testing.T) {
	parent := t.TempDir()
	_, err := NewProject(parent, "Dupe", "Author", "none")
	if err != nil {
		t.Fatalf("first creation failed: %v", err)
	}

	_, err = NewProject(parent, "Dupe", "Author", "none")
	if err == nil {
		t.Error("expected error when creating duplicate project")
	}
}

func TestOpenProject(t *testing.T) {
	parent := t.TempDir()
	created, err := NewProject(parent, "Open Test", "Author", "zettelkasten")
	if err != nil {
		t.Fatalf("NewProject failed: %v", err)
	}

	// Open from root.
	opened, err := OpenProject(created.RootDir)
	if err != nil {
		t.Fatalf("OpenProject failed: %v", err)
	}
	if opened.Config.Project.Title != "Open Test" {
		t.Errorf("expected title 'Open Test', got %q", opened.Config.Project.Title)
	}

	// Open from a subdirectory (should discover project.toml upward).
	sub := filepath.Join(created.RootDir, "inbox")
	opened2, err := OpenProject(sub)
	if err != nil {
		t.Fatalf("OpenProject from subdirectory failed: %v", err)
	}
	if opened2.RootDir != created.RootDir {
		t.Errorf("expected root %q, got %q", created.RootDir, opened2.RootDir)
	}
}

func TestCreateAndListNotes(t *testing.T) {
	parent := t.TempDir()
	proj, err := NewProject(parent, "Notes Test", "Author", "zettelkasten")
	if err != nil {
		t.Fatalf("NewProject failed: %v", err)
	}

	// Create notes in different directories.
	rel1, err := proj.CreateNote("Trust as Social Contract", "permanent")
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}
	if rel1 != filepath.Join("permanent", "trust-as-social-contract.md") {
		t.Errorf("unexpected path: %s", rel1)
	}

	rel2, err := proj.CreateNote("Quick Thought", "inbox")
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	// List should find both notes.
	files, err := proj.ListMarkdownFiles()
	if err != nil {
		t.Fatalf("ListMarkdownFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
	_ = rel2
}

func TestReadWriteNote(t *testing.T) {
	parent := t.TempDir()
	proj, err := NewProject(parent, "RW Test", "Author", "none")
	if err != nil {
		t.Fatalf("NewProject failed: %v", err)
	}

	relPath, err := proj.CreateNote("Test Note", "")
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	content, err := proj.ReadNote(relPath)
	if err != nil {
		t.Fatalf("ReadNote failed: %v", err)
	}
	if content == "" {
		t.Error("expected non-empty content from freshly created note")
	}

	newContent := "---\ntitle: \"Updated\"\n---\n\nNew body text.\n"
	if err := proj.WriteNote(relPath, newContent); err != nil {
		t.Fatalf("WriteNote failed: %v", err)
	}

	read, err := proj.ReadNote(relPath)
	if err != nil {
		t.Fatalf("ReadNote after write failed: %v", err)
	}
	if read != newContent {
		t.Errorf("expected updated content, got %q", read)
	}
}

func TestRenameNote(t *testing.T) {
	parent := t.TempDir()
	proj, err := NewProject(parent, "Rename Test", "Author", "none")
	if err != nil {
		t.Fatalf("NewProject failed: %v", err)
	}

	relPath, err := proj.CreateNote("Old Name", "")
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	newRel, err := proj.RenameNote(relPath, "New Name")
	if err != nil {
		t.Fatalf("RenameNote failed: %v", err)
	}
	if newRel != "new-name.md" {
		t.Errorf("expected 'new-name.md', got %q", newRel)
	}

	// Old file should not exist.
	if _, err := os.Stat(filepath.Join(proj.RootDir, relPath)); !os.IsNotExist(err) {
		t.Error("old file should not exist after rename")
	}

	// New file should exist.
	if _, err := os.Stat(filepath.Join(proj.RootDir, newRel)); err != nil {
		t.Error("new file should exist after rename")
	}
}

func TestDeleteNote(t *testing.T) {
	parent := t.TempDir()
	proj, err := NewProject(parent, "Delete Test", "Author", "none")
	if err != nil {
		t.Fatalf("NewProject failed: %v", err)
	}

	relPath, err := proj.CreateNote("To Delete", "")
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	if err := proj.DeleteNote(relPath); err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(proj.RootDir, relPath)); !os.IsNotExist(err) {
		t.Error("file should not exist after delete")
	}
}

func TestSanitizeDirName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Knowledge Base", "my-knowledge-base"},
		{"Hello World!!!", "hello-world"},
		{"  spaces  ", "spaces"},
		{"CamelCase", "camelcase"},
	}
	for _, tt := range tests {
		got := sanitizeDirName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeDirName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
