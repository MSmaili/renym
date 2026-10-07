package main

import (
	"github.com/MSmaili/renym/internal/log"
	"github.com/MSmaili/renym/internal/templates"
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Inspect rename and organization templates",
}

var validateTemplateCmd = &cobra.Command{
	Use:   "validate <name-or-file>",
	Short: "Validate a TOML/YAML template without discovering or renaming files",
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
		status := ""
		if compiled.HasMoves() {
			status = " (organization; explicit --path and apply history required)"
		}
		log.Info("Valid template: %s (%d rule(s), kind=%s, recursive=%t)%s\n", compiled.SourcePath(), len(spec.Rules), spec.Selection.Kind, spec.Selection.Recursive, status)
		return nil
	},
}

var listTemplateCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured template names and origin paths",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		catalog, err := templates.List()
		if err != nil {
			return err
		}
		if len(catalog.Templates) == 0 {
			log.Info("No templates found in %s\n", catalog.Directory)
			return nil
		}
		for _, reference := range catalog.Templates {
			status := ""
			if reference.Ambiguous {
				status = " (ambiguous; use explicit path)"
			}
			log.Info("%s\t%s%s\n", reference.Name, reference.Path, status)
		}
		return nil
	},
}

func init() {
	templateCmd.AddCommand(validateTemplateCmd)
	templateCmd.AddCommand(listTemplateCmd)
	rootCmd.AddCommand(templateCmd)
}
