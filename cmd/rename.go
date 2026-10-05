package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MSmaili/renym/internal/app"
	"github.com/MSmaili/renym/internal/cli"
	"github.com/MSmaili/renym/internal/engine"
	"github.com/MSmaili/renym/internal/fs"
	"github.com/MSmaili/renym/internal/history"
	"github.com/MSmaili/renym/internal/log"
	"github.com/MSmaili/renym/internal/version"
	"github.com/spf13/cobra"
)

var (
	mode            string
	templatePath    string
	path            string
	recursive       bool
	directories     bool
	dirsOnly        bool
	ignore          []string
	noDefaultIgnore bool
	skipHistory     bool
	showVersion     bool
)

func init() {
	// Path flags
	rootCmd.Flags().StringVarP(&path, "path", "p", ".", "Path to directory or file")

	// Traversal flags
	rootCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively rename in subdirectories")

	// dirs flags
	rootCmd.Flags().BoolVarP(&directories, "directories", "d", false, "Include directories in rename (default = false)")
	rootCmd.Flags().BoolVarP(&dirsOnly, "dirs-only", "D", false, "Rename only directories, skip files (default = false)")

	// Filter flags
	rootCmd.Flags().StringSliceVar(&ignore, "ignore", nil, "Glob pattern to ignore (can be specified multiple times)")
	rootCmd.Flags().BoolVar(&noDefaultIgnore, "no-default-ignore", false, "Disable default ignore patterns (.git, .svn, .hg)")

	// Backup
	rootCmd.Flags().BoolVarP(&skipHistory, "skip-history", "", false, "Skip adding a json file for operation history which can be used for undo")

	// Version
	rootCmd.Flags().BoolVarP(&showVersion, "version", "V", false, "Show the current installed version")

	// Modes  flags
	rootCmd.Flags().StringVarP(&mode, "mode", "m", "", "Rename mode: upper, lower, pascal, camel, snake, kebab, title")
	rootCmd.Flags().StringVar(&templatePath, "template", "", "TOML/YAML template name or file path (exclusive with --mode)")
	rootCmd.RegisterFlagCompletionFunc("mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"upper", "lower", "pascal", "camel", "snake", "kebab", "title"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		msg := err.Error()

		if strings.Contains(msg, "flag needs an argument") && (strings.HasSuffix(msg, "-m") || strings.HasSuffix(msg, "--mode")) {
			log.Error("The --mode flag requires a value.\n")
			log.Error("Available modes: upper, lower, pascal, camel, snake, kebab, title\n")
			log.Error("\nRun renym --help for more info\n")
			os.Exit(1)
		}

		return err
	})
}

func validateFlags(cmd *cobra.Command, args []string) error {
	if showVersion {
		log.Print("renym version %s\n", version.Version)
		os.Exit(0)
	}
	if cmd.Flags().Changed("template") {
		if cmd.Flags().Changed("mode") {
			return fmt.Errorf("%w: --mode and --template cannot be used together", cli.ErrConflictingFlags)
		}
		if templatePath == "" {
			return fmt.Errorf("--template requires a template name or file path")
		}
		// The application validates the preset before it discovers the input path.
		return nil
	}
	if !cmd.Flags().Changed("mode") {
		_ = cmd.Help()
		os.Exit(0)
	}
	return cli.ValidateFlags(mode, path)
}

func runRename(cmd *cobra.Command, args []string) error {
	cfg := app.Request{
		Path:            path,
		Mode:            mode,
		TemplatePath:    templatePath,
		Recursive:       recursive,
		Directories:     directories || dirsOnly,
		Files:           !dirsOnly,
		Ignore:          ignore,
		NoDefaultIgnore: noDefaultIgnore,
		SkipHistory:     skipHistory,
		DryRun:          globalCfg.DryRun,
		Command:         strings.Join(os.Args, " "),
		Version:         version.Version,
	}
	if templatePath != "" {
		cfg.SelectionOverrides = templateSelectionOverrides(cmd)
	}

	adapter := fs.NewAdapter()

	service := app.NewService(adapter, nil)
	// Constructing the store only resolves its location; planning/preview never
	// read or write its records. This also lets the service protect journal paths.
	if !cfg.SkipHistory {
		store, err := history.NewGlobalStore(adapter)
		if err != nil && !cfg.DryRun {
			return fmt.Errorf("history is required: %w", err)
		}
		if err == nil {
			service = app.NewService(adapter, store)
		}
	}
	plan, err := service.Plan(cmd.Context(), cfg)
	if err != nil {
		return err
	}
	if plan.TemplatePath != "" {
		log.Info("Template: %s", plan.TemplatePath)
		if plan.TemplateName != "" {
			log.Info(" (%s)", plan.TemplateName)
		}
		log.Info("\nSelection: kind=%s recursive=%t ignore=%v no_default_ignore=%t\n", plan.Selection.Kind, plan.Selection.Recursive, plan.Selection.Ignore, plan.Selection.NoDefaultIgnore)
		if len(plan.Overrides) > 0 {
			log.Info("CLI overrides: %s\n", strings.Join(plan.Overrides, ", "))
		}
		for _, match := range plan.Matches {
			action := match.Mode
			if match.Filename {
				action = "filename"
			}
			log.Debug("Rule %s (%s, index=%d): %s\n", match.RuleID, action, match.Index, match.Path)
		}
	}

	if len(plan.Result.Operations) == 0 {
		log.Info("\n✓ No files to rename\n")
		printSkipped(plan.Result.Skipped)
		return nil
	}

	log.Debug("Processing %d file(s)...\n", len(plan.Result.Operations))

	result, err := service.Execute(cmd.Context(), plan)
	if err != nil {
		return fmt.Errorf("rename failed after %d completed operation(s): %w", len(result.Execution.Completed), err)
	}

	if cfg.DryRun {
		for _, op := range plan.Result.Operations {
			log.Info("Would rename: %s -> %s\n", op.OldPath, op.NewPath)
		}
	}
	printResults(plan.Result, cfg.DryRun)

	return nil
}

func templateSelectionOverrides(cmd *cobra.Command) app.SelectionOverrides {
	var overrides app.SelectionOverrides
	if cmd.Flags().Changed("directories") || cmd.Flags().Changed("dirs-only") {
		kind := "files"
		if dirsOnly {
			kind = "directories"
		} else if directories {
			kind = "both"
		}
		overrides.Kind = &kind
	}
	if cmd.Flags().Changed("recursive") {
		value := recursive
		overrides.Recursive = &value
	}
	if cmd.Flags().Changed("ignore") {
		value := append([]string(nil), ignore...)
		overrides.Ignore = &value
	}
	if cmd.Flags().Changed("no-default-ignore") {
		value := noDefaultIgnore
		overrides.NoDefaultIgnore = &value
	}
	return overrides
}

func printResults(result engine.PlanResult, dryRun bool) {
	separator := strings.Repeat("=", 60)
	thinSeparator := strings.Repeat("-", 60)

	// Success header
	log.Info("\n%s\n", separator)
	if dryRun {
		log.Info("  DRY RUN - No files were actually renamed\n")
	} else {
		log.Info("  ✓ COMPLETED SUCCESSFULLY\n")
		log.Info("%s\n", separator)
		log.Info("  Files renamed:   %d\n", len(result.Operations))
	}

	// Show warnings if any
	if len(result.Skipped) > 0 {
		log.Info("  Files skipped:   %d\n", len(result.Skipped))
	}
	if len(result.Collisions) > 0 {
		log.Info("  Collisions:      %d\n", len(result.Collisions))
	}
	log.Info("%s\n", separator)

	// Show collision details
	if len(result.Collisions) > 0 {
		log.Info("\n⚠ COLLISIONS:\n")
		log.Info("%s\n", thinSeparator)
		for i, collision := range result.Collisions {
			log.Info("  %d. Multiple files trying to rename to:\n", i+1)
			log.Info("     → %s\n", filepath.Base(collision.Target))
			log.Info("     Sources: %s, %s\n", filepath.Base(collision.Source1), filepath.Base(collision.Source2))
			if i < len(result.Collisions)-1 {
				log.Info("\n")
			}
		}
		log.Info("%s\n", thinSeparator)
	}

	printSkipped(result.Skipped)
	log.Info("\n")
}

func printSkipped(skipped []engine.SkippedFile) {
	for _, item := range skipped {
		log.Info("Skipped: %s (%s)\n", item.Path, item.Reason)
	}
}
