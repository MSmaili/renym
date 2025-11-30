package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "rnm [flags]",
	Short:   "Fast, safe, cross-platform file rename tool",
	Long:    `A CLI tool for renaming your files. It supports different modes, and recursive renaming`,
	PreRunE: validateFlags,
	RunE:    runRename,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
