// Bragi — a terminal knowledge base with semantic search.
package main

import (
	"fmt"
	"os"

	"github.com/adambick/bragi/internal/knowledgebase"
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
	root.AddCommand(versionCmd())

	return root
}

func newCmd() *cobra.Command {
	var methodology string
	var author string

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
			return nil
		},
	}

	cmd.Flags().StringVarP(&methodology, "methodology", "m", "none",
		`knowledge base methodology (para|zettelkasten|johnny-decimal|none)`)
	cmd.Flags().StringVarP(&author, "author", "a", "",
		`project author name`)

	return cmd
}

func openCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [path]",
		Short: "Open a Bragi project in the TUI",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}
			return launchTUI(path)
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
