package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var manOutDir string

var manCmd = &cobra.Command{
	Use:   "man",
	Short: "Generate man pages for rpcduel into a directory",
	Long: `Generate roff(1) man pages for rpcduel and all its subcommands.

The output directory is created if it does not exist. One file is written per
command, e.g. rpcduel.1, rpcduel-bench.1, rpcduel-duel.1, ...`,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if manOutDir == "" {
			return fmt.Errorf("--dir is required")
		}
		if err := os.MkdirAll(manOutDir, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}
		header := &doc.GenManHeader{
			Title:   "RPCDUEL",
			Section: "1",
			Source:  "rpcduel " + buildVersion,
			Manual:  "rpcduel Manual",
		}
		if err := doc.GenManTree(rootCmd, header, manOutDir); err != nil {
			return fmt.Errorf("generate man pages: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote man pages to %s\n", filepath.Clean(manOutDir))
		return nil
	},
}

func init() {
	manCmd.Flags().StringVar(&manOutDir, "dir", "", "Output directory for generated man pages")
	rootCmd.AddCommand(manCmd)
}
