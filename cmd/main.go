package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rnm [flags]",
	Short: "Fast, safe, cross-platform file rename tool",
	Long: `Rename files and directories using automatic naming patterns.

Modes:
  upper   → FILENAME
  lower   → filename
  pascal  → FileName
  camel   → fileName
  snake   → file_name
  kebab   → file-name
  title   → File Name`,
	Example: `
  rnm -m upper
  rnm -m snake -p ./photos
  rnm -m kebab --dry-run`,
	PreRunE: validateFlags,
	RunE:    runRename,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
