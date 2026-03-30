// Bragi — a terminal knowledge base with semantic search.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adambick/bragi/internal/database"
	"github.com/adambick/bragi/internal/embedding"
	"github.com/adambick/bragi/internal/knowledgebase"
	"github.com/adambick/bragi/internal/registry"
	"github.com/adambick/bragi/internal/search"
	"github.com/adambick/bragi/internal/tui"
	"github.com/spf13/cobra"
)

// version is set at build time via ldflags.
var version = "dev"

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "bragi",
		Short: "A terminal knowledge base with semantic search",
		Long:  "Bragi is a TUI knowledge base editor with built-in semantic search,\nwikilink cross-referencing, and methodology-aware organization.",
		// No args = launch TUI in current directory.
		RunE: func(cmd *cobra.Command, args []string) error {
			return launchTUI(".")
		},
	}

	root.AddCommand(newCmd())
	root.AddCommand(openCmd())
	root.AddCommand(indexCmd())
	root.AddCommand(listCmd())
	root.AddCommand(addCmd())
	root.AddCommand(removeCmd())
	root.AddCommand(versionCmd())

	return root
}

func newCmd() *cobra.Command {
	var methodology string
	var author string
	var noRegister bool

	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new Bragi project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := args[0]
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}

			proj, err := knowledgebase.NewProject(cwd, title, author, methodology)
			if err != nil {
				return err
			}

			fmt.Printf("Created project %q at %s\n", title, proj.RootDir)
			fmt.Printf("Methodology: %s\n", methodology)

			// Auto-register in the global registry.
			if !noRegister {
				name := registry.NameFromTitle(title)
				if regErr := registerProject(name, proj.RootDir); regErr != nil {
					fmt.Fprintf(os.Stderr, "Note: could not register project: %v\n", regErr)
				} else {
					fmt.Printf("Registered as %q (open with: bragi open %s)\n", name, name)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&methodology, "methodology", "m", "none",
		`knowledge base methodology (para|zettelkasten|johnny-decimal|none)`)
	cmd.Flags().StringVarP(&author, "author", "a", "",
		`project author name`)
	cmd.Flags().BoolVar(&noRegister, "no-register", false,
		"do not add the new project to the global registry")

	return cmd
}

func openCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [name-or-path]",
		Short: "Open a Bragi project in the TUI",
		Long: "Open a Bragi project by registered name or filesystem path.\n\n" +
			"Examples:\n" +
			"  bragi open               # open project in current directory\n" +
			"  bragi open my-notes      # open by registry name\n" +
			"  bragi open ~/notes       # open by path",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				resolved, err := registry.ResolvePath(args[0])
				if err != nil {
					return fmt.Errorf("resolving project: %w", err)
				}
				path = resolved
			}
			return launchTUI(path)
		},
	}
}

func indexCmd() *cobra.Command {
	var downloadModel bool

	cmd := &cobra.Command{
		Use:   "index [path]",
		Short: "Reindex all files in a Bragi project",
		Long:  "Reindex all markdown files for semantic search.\nDownloads the embedding model on first run.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			proj, err := knowledgebase.OpenProject(path)
			if err != nil {
				return fmt.Errorf("could not open project: %w", err)
			}

			// Open database.
			db, err := database.Open(proj.RootDir)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := db.Migrate(); err != nil {
				return fmt.Errorf("migrating database: %w", err)
			}

			// Ensure model.
			modelName := proj.Config.Search.EmbeddingModel
			if modelName == "" {
				modelName = "nomic-ai/nomic-embed-text-v1.5"
			}

			modelCacheDir, err := embedding.ModelCacheDir(modelName)
			if err != nil {
				return fmt.Errorf("resolving model cache dir: %w", err)
			}

			if downloadModel {
				// Force re-download by removing stale cache.
				fmt.Printf("Removing cached model at %s…\n", modelCacheDir)
				if err := os.RemoveAll(modelCacheDir); err != nil {
					return fmt.Errorf("clearing model cache: %w", err)
				}
			}

			if !embedding.ModelCached(modelCacheDir) {
				fmt.Printf("Downloading model %s…\n", modelName)
			}

			// Stream progress to stdout.
			progressCh := make(chan embedding.DownloadProgress, 32)
			go func() {
				lastFile := ""
				for p := range progressCh {
					if p.Done {
						fmt.Printf("\rDownload complete.                        \n")
						return
					}
					if p.Err != nil {
						fmt.Fprintf(os.Stderr, "\rDownload error: %v\n", p.Err)
						return
					}
					if p.File != lastFile {
						if lastFile != "" {
							fmt.Println()
						}
						fmt.Printf("  %s", p.File)
						lastFile = p.File
					}
					if p.TotalBytes > 0 {
						pct := 100 * p.BytesDownloaded / p.TotalBytes
						fmt.Printf("\r  %s … %d%%", p.File, pct)
					}
				}
			}()

			modelDir, err := embedding.EnsureModel(modelName, progressCh)
			close(progressCh)
			if err != nil {
				return fmt.Errorf("model not available: %w\nRun 'bragi index --download-model' to force a fresh download.", err)
			}

			embedder, err := embedding.NewONNXProvider(modelDir)
			if err != nil {
				return fmt.Errorf("ONNX Runtime not available: %w", err)
			}
			defer embedder.Close()

			// Index all files.
			indexer := search.NewIndexer(db, embedder, proj, proj.Config.Search)
			indexer.Start()
			defer indexer.Stop()

			files, err := proj.ListMarkdownFiles()
			if err != nil {
				return fmt.Errorf("listing files: %w", err)
			}

			fmt.Printf("Indexing %d files...\n", len(files))
			for _, f := range files {
				indexer.Enqueue(search.IndexRequest{RelPath: f})
			}

			// Wait for all files to be processed.
			processed := 0
			for processed < len(files) {
				select {
				case status := <-indexer.Status():
					processed++
					if status.Err != nil {
						fmt.Fprintf(os.Stderr, "  Error: %s: %v\n", status.RelPath, status.Err)
					}
				case <-context.Background().Done():
					return nil
				}
			}

			fmt.Printf("Indexed %d files successfully.\n", processed)
			return nil
		},
	}

	cmd.Flags().BoolVar(&downloadModel, "download-model", false,
		"Download the embedding model if not cached")

	return cmd
}

// listCmd shows all registered projects.
func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered Bragi projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			entries := reg.List()
			if len(entries) == 0 {
				fmt.Println("No projects registered.")
				fmt.Println("Use 'bragi add <name> <path>' or 'bragi new <title>' to register one.")
				return nil
			}

			// Find the longest name for alignment.
			maxLen := 0
			for _, e := range entries {
				if len(e.Name) > maxLen {
					maxLen = len(e.Name)
				}
			}

			fmt.Println("Registered projects:")
			for _, e := range entries {
				fmt.Printf("  %-*s  %s\n", maxLen, e.Name, e.Path)
			}
			return nil
		},
	}
}

// addCmd registers an existing project in the global registry.
func addCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> [path]",
		Short: "Register a project in the global registry",
		Long: "Register an existing Bragi project so it can be opened by name.\n\n" +
			"If path is omitted, the current directory is used.\n\n" +
			"Example:\n" +
			"  bragi add work-kb ~/work/knowledge-base",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			path := "."
			if len(args) > 1 {
				path = args[1]
			}

			abs, err := absPath(path)
			if err != nil {
				return err
			}

			// Verify it's a valid Bragi project.
			if _, err := knowledgebase.OpenProject(abs); err != nil {
				return fmt.Errorf("%s does not appear to be a Bragi project: %w", abs, err)
			}

			if err := registerProject(name, abs); err != nil {
				return err
			}

			fmt.Printf("Registered %q → %s\n", name, abs)
			fmt.Printf("Open with: bragi open %s\n", name)
			return nil
		},
	}
}

// removeCmd removes a project from the global registry.
func removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a project from the global registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			if err := reg.Remove(name); err != nil {
				return err
			}

			if err := reg.Save(); err != nil {
				return fmt.Errorf("saving registry: %w", err)
			}

			fmt.Printf("Removed %q from registry.\n", name)
			return nil
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Bragi version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("bragi %s\n", version)
		},
	}
}

// launchTUI opens a project and starts the TUI.
func launchTUI(path string) error {
	proj, err := knowledgebase.OpenProject(path)
	if err != nil {
		return fmt.Errorf("could not open project: %w\n\nRun 'bragi new <title>' to create a project first.", err)
	}
	return tui.Run(proj)
}

// registerProject loads (or creates) the global registry, adds or updates the
// entry, and saves.  Best-effort: callers should handle the error gracefully.
func registerProject(name, path string) error {
	reg, err := registry.Load()
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	if err := reg.AddOrUpdate(name, path); err != nil {
		return err
	}

	return reg.Save()
}

// absPath returns the absolute version of the given path.
func absPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path %s: %w", path, err)
	}
	return abs, nil
}
