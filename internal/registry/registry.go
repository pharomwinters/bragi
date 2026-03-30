// Package registry manages a global list of known Bragi projects so that
// users can open any project by name from any directory.
//
// The registry is stored at ~/.config/bragi/registry.toml:
//
//	[projects]
//	my-notes = "/home/alice/notes"
//	work     = "/home/alice/work/kb"
package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const configFileName = "registry.toml"

// Entry is a single registered project.
type Entry struct {
	Name string
	Path string
}

// Registry is the in-memory representation of the registry file.
type Registry struct {
	path     string            // absolute path to the TOML file
	Projects map[string]string `toml:"projects"` // name → absolute path
}

// GlobalPath returns the path to the global registry file
// (~/.config/bragi/registry.toml).
func GlobalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".config", "bragi", configFileName), nil
}

// Load reads the registry from disk, creating an empty one if the file does
// not yet exist.
func Load() (*Registry, error) {
	p, err := GlobalPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(p)
}

// LoadFrom reads the registry from a specific path.
func LoadFrom(path string) (*Registry, error) {
	r := &Registry{
		path:     path,
		Projects: make(map[string]string),
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil // fresh registry — nothing to parse
	}
	if err != nil {
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	if err := toml.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parsing registry: %w", err)
	}

	if r.Projects == nil {
		r.Projects = make(map[string]string)
	}

	return r, nil
}

// Save writes the registry back to disk.
func (r *Registry) Save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := toml.Marshal(r)
	if err != nil {
		return fmt.Errorf("encoding registry: %w", err)
	}

	if err := os.WriteFile(r.path, data, 0o644); err != nil {
		return fmt.Errorf("writing registry: %w", err)
	}

	return nil
}

// Add registers a project under the given name.  The path is stored as an
// absolute path.  Returns an error if the name is already taken (use Remove
// first to rename).
func (r *Registry) Add(name, path string) error {
	if name == "" {
		return errors.New("project name must not be empty")
	}
	if _, exists := r.Projects[name]; exists {
		return fmt.Errorf("project %q is already registered at %s", name, r.Projects[name])
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	r.Projects[name] = abs
	return nil
}

// AddOrUpdate adds or overwrites a project entry without returning an error
// if the name already exists.  Used for auto-registration.
func (r *Registry) AddOrUpdate(name, path string) error {
	if name == "" {
		return errors.New("project name must not be empty")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	r.Projects[name] = abs
	return nil
}

// Remove deletes a project from the registry by name.
func (r *Registry) Remove(name string) error {
	if _, exists := r.Projects[name]; !exists {
		return fmt.Errorf("project %q not found in registry", name)
	}
	delete(r.Projects, name)
	return nil
}

// Resolve returns the filesystem path for a project name.  Returns
// ("", false) when the name is not registered.
func (r *Registry) Resolve(name string) (string, bool) {
	path, ok := r.Projects[name]
	return path, ok
}

// List returns all entries sorted by name.
func (r *Registry) List() []Entry {
	entries := make([]Entry, 0, len(r.Projects))
	for name, path := range r.Projects {
		entries = append(entries, Entry{Name: name, Path: path})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// ResolvePath takes a user-supplied argument and returns a filesystem path.
// The lookup order is:
//  1. Exact name match in the registry.
//  2. Case-insensitive name match.
//  3. Treat the argument as a file-system path (returned as-is).
func ResolvePath(arg string) (string, error) {
	r, err := Load()
	if err != nil {
		// Registry unreadable — fall back to treating arg as a path.
		return arg, nil
	}

	// 1. Exact name match.
	if path, ok := r.Resolve(arg); ok {
		return path, nil
	}

	// 2. Case-insensitive match.
	lower := strings.ToLower(arg)
	for name, path := range r.Projects {
		if strings.ToLower(name) == lower {
			return path, nil
		}
	}

	// 3. Treat as a filesystem path.
	return arg, nil
}

// NameFromTitle derives a registry-safe name from a project title by
// lower-casing and replacing spaces/special characters with hyphens.
func NameFromTitle(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' {
			b.WriteRune('-')
		}
	}
	// Collapse consecutive hyphens.
	result := b.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}
