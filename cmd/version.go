package main

import (
	"fmt"

	"github.com/MSmaili/renym/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the version of renym.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("renym version %s\n", version.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	hideGlobalFlags(versionCmd)
}
