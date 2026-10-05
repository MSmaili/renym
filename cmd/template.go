package main

import (
	"github.com/MSmaili/renym/internal/log"
	"github.com/MSmaili/renym/internal/templates"
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Inspect rename presets",
}

var validateTemplateCmd = &cobra.Command{
	Use:   "validate <file.toml>",
	Short: "Validate an explicit TOML preset without discovering or renaming files",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		compiled, err := templates.Load(args[0])
		if err != nil {
			return err
		}
		spec := compiled.Snapshot()
		log.Info("Valid template: %s (%d rule(s), kind=%s, recursive=%t)\n", args[0], len(spec.Rules), spec.Selection.Kind, spec.Selection.Recursive)
		return nil
	},
}

func init() {
	templateCmd.AddCommand(validateTemplateCmd)
	rootCmd.AddCommand(templateCmd)
}
