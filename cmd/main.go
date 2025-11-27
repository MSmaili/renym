package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rnm [flags]",
	Short: "Fast, safe, cross-platform file rename tool",
	Long:  `rename your beautiful files boyy`,
	RunE:  runRename,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
