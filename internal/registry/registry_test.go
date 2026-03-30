package registry_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adambick/bragi/internal/registry"
)

func TestLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.toml")

	reg, err := registry.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if len(reg.List()) != 0 {
		t.Errorf("expected empty registry, got %d entries", len(reg.List()))
	}
}

func TestAddAndResolve(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.toml")

	reg, _ := registry.LoadFrom(path)
	if err := reg.Add("my-notes", "/home/alice/notes"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	p, ok := reg.Resolve("my-notes")
	if !ok {
		t.Fatal("Resolve returned false")
	}
	if p != "/home/alice/notes" {
		t.Errorf("got %q, want /home/alice/notes", p)
	}
}

func TestAddDuplicate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.toml")

	reg, _ := registry.LoadFrom(path)
	_ = reg.Add("notes", "/a")
	err := reg.Add("notes", "/b")
	if err == nil {
		t.Fatal("expected error on duplicate Add, got nil")
	}
}

func TestAddOrUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.toml")

	reg, _ := registry.LoadFrom(path)
	_ = reg.Add("notes", "/a")
	if err := reg.AddOrUpdate("notes", "/b"); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}
	p, _ := reg.Resolve("notes")
	if p != "/b" {
		t.Errorf("expected /b, got %q", p)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.toml")

	reg, _ := registry.LoadFrom(path)
	_ = reg.Add("notes", "/a")
	if err := reg.Remove("notes"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := reg.Resolve("notes"); ok {
		t.Error("entry still present after Remove")
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.toml")

	reg, _ := registry.LoadFrom(path)
	_ = reg.Add("kb", "/home/bob/kb")
	if err := reg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reg2, err := registry.LoadFrom(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	p, ok := reg2.Resolve("kb")
	if !ok || p != "/home/bob/kb" {
		t.Errorf("after reload, got (%q, %v)", p, ok)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.toml")

	reg, _ := registry.LoadFrom(path)
	_ = reg.Add("beta", "/b")
	_ = reg.Add("alpha", "/a")

	entries := reg.List()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Should be sorted.
	if entries[0].Name != "alpha" || entries[1].Name != "beta" {
		t.Errorf("unexpected order: %v", entries)
	}
}

func TestNameFromTitle(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{"My Notes", "my-notes"},
		{"Work KB!", "work-kb"},
		{"UPPERCASE", "uppercase"},
		{"hello--world", "hello-world"},
		{"  leading spaces  ", "leading-spaces"},
	}
	for _, c := range cases {
		got := registry.NameFromTitle(c.title)
		if got != c.want {
			t.Errorf("NameFromTitle(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

func TestResolvePath_FallbackToFS(t *testing.T) {
	// Use a temp dir for the config so we don't touch the real registry.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create a dummy directory to resolve.
	dir := t.TempDir()

	got, err := registry.ResolvePath(dir)
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	// Should return the path unchanged (not in registry → fallback to FS).
	if got != dir {
		t.Errorf("expected %q, got %q", dir, got)
	}
}

func TestGlobalPath(t *testing.T) {
	p, err := registry.GlobalPath()
	if err != nil {
		t.Fatalf("GlobalPath: %v", err)
	}
	// Must end with the config file name.
	if filepath.Base(p) != "registry.toml" {
		t.Errorf("unexpected file name: %s", p)
	}
	// Must be inside a .config/bragi directory.
	if filepath.Base(filepath.Dir(p)) != "bragi" {
		t.Errorf("unexpected parent dir: %s", filepath.Dir(p))
	}
}

// TestSaveCreatesDir verifies that Save creates the config directory tree.
func TestSaveCreatesDir(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "deep", "nested", "registry.toml")

	reg, err := registry.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	_ = reg.Add("test", "/x")
	if err := reg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
