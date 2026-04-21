package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var (
	docsOutputDir        string
	docsEnableAutoGenTag bool
)

var docsCmd = &cobra.Command{
	Use:    "docs",
	Short:  "Generate documentation for booba",
	Hidden: true,
	Long: `Generate documentation for all booba commands.

Subcommands:
  markdown  Generate plain markdown (default)
  man       Generate man pages

The auto-generation tag (timestamp footer) is disabled by default for
stable, reproducible files. Pass --enableAutoGenTag when publishing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDocsMarkdown(cmd, args)
	},
}

var docsMarkdownCmd = &cobra.Command{
	Use:   "markdown",
	Short: "Generate markdown documentation",
	Long:  `Generate plain markdown documentation for all booba commands.`,
	RunE:  runDocsMarkdown,
}

var docsManCmd = &cobra.Command{
	Use:   "man",
	Short: "Generate man pages",
	Long: `Generate man pages in roff format for all booba commands.

Output is suitable for installation in /usr/share/man/man1 or
/usr/local/share/man/man1.`,
	RunE: runDocsMan,
}

func init() {
	docsCmd.PersistentFlags().StringVarP(&docsOutputDir, "output", "o", "docs", "output directory")
	docsCmd.PersistentFlags().BoolVar(&docsEnableAutoGenTag, "enableAutoGenTag", false, "include auto-generated timestamp footer")
	docsCmd.AddCommand(docsMarkdownCmd, docsManCmd)
}

func runDocsMarkdown(cmd *cobra.Command, _ []string) error {
	if err := os.MkdirAll(docsOutputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	rootCmd.DisableAutoGenTag = !docsEnableAutoGenTag
	if err := doc.GenMarkdownTree(rootCmd, docsOutputDir); err != nil {
		return fmt.Errorf("generate markdown: %w", err)
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "Generated %d markdown files in %s\n", countFiles(docsOutputDir, ".md"), docsOutputDir)
	return err
}

func runDocsMan(cmd *cobra.Command, _ []string) error {
	if err := os.MkdirAll(docsOutputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	rootCmd.DisableAutoGenTag = !docsEnableAutoGenTag
	header := &doc.GenManHeader{Title: "BOOBA", Section: "1"}
	if err := doc.GenManTree(rootCmd, header, docsOutputDir); err != nil {
		return fmt.Errorf("generate man pages: %w", err)
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "Generated %d man pages in %s\n", countFiles(docsOutputDir, ".1"), docsOutputDir)
	return err
}

func countFiles(dir, ext string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var n int
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ext {
			n++
		}
	}
	return n
}
