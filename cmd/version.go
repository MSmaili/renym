package main

import (
	"fmt"

	"github.com/MSmaili/rnm/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the version of rnm.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("rnm version %s\n", version.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
