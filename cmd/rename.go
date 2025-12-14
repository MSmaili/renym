package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MSmaili/rnm/internal/cli"
	"github.com/MSmaili/rnm/internal/common"
	"github.com/MSmaili/rnm/internal/engine"
	"github.com/MSmaili/rnm/internal/fs"
	"github.com/MSmaili/rnm/internal/history"
	"github.com/MSmaili/rnm/internal/log"
	"github.com/MSmaili/rnm/internal/version"
	"github.com/MSmaili/rnm/internal/walker"
	"github.com/spf13/cobra"
)

var (
	mode            string
	path            string
	recursive       bool
	directories     bool
	dirsOnly        bool
	ignore          []string
	noDefaultIgnore bool
	dryRun          bool
	skipHistory     bool
	showVersion     bool
)

type Config struct {
	Path            string
	Mode            string
	Recursive       bool
	Directories     bool
	Files           bool
	Ignore          []string
	NoDefaultIgnore bool
	DryRun          bool
	SkipHistory     bool
}

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

	// Output flags
	rootCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would be renamed without actually renaming")

	// Backup
	rootCmd.Flags().BoolVarP(&skipHistory, "skip-history", "", false, "Skip adding a json file for operation history which can be used for undo")

	// Version
	rootCmd.Flags().BoolVarP(&showVersion, "version", "V", false, "Show the current installed version")

	// Modes  flags
	rootCmd.Flags().StringVarP(&mode, "mode", "m", "", "Rename mode: upper, lower, pascal, camel, snake, kebab, title")
	rootCmd.RegisterFlagCompletionFunc("mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"upper", "lower", "pascal", "camel", "snake", "kebab", "title"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		msg := err.Error()

		if strings.Contains(msg, "flag needs an argument") && (strings.HasSuffix(msg, "-m") || strings.HasSuffix(msg, "--mode")) {
			log.Error("The --mode flag requires a value.\n")
			log.Error("Available modes: upper, lower, pascal, camel, snake, kebab, title\n")
			log.Error("\nRun rnm --help for more info\n")
			os.Exit(1)
		}

		return err
	})
}

func validateFlags(cmd *cobra.Command, args []string) error {
	if showVersion {
		log.Print("rnm version %s\n", version.Version)
		os.Exit(0)
	}
	if !cmd.Flags().Changed("mode") {
		_ = cmd.Help()
		os.Exit(0)
	}
	return cli.ValidateFlags(mode, path)
}

func runRename(cmd *cobra.Command, args []string) error {
	cfg := Config{
		Path:            path,
		Mode:            mode,
		Recursive:       recursive,
		Directories:     directories || dirsOnly,
		Files:           !dirsOnly,
		Ignore:          ignore,
		NoDefaultIgnore: noDefaultIgnore,
		SkipHistory:     skipHistory,
		DryRun:          dryRun,
	}

	adapter := fs.NewAdapter()

	pathsToRename, err := walker.Walk(walker.Config{
		Path:            cfg.Path,
		Recursive:       cfg.Recursive,
		Directories:     cfg.Directories,
		NoDefaultIgnore: cfg.NoDefaultIgnore,
		Files:           cfg.Files,
		Ignore:          cfg.Ignore,
	})
	if err != nil {
		return err
	}

	renameMode := engine.ModeRegistry[cfg.Mode]
	engine := engine.NewEngine(renameMode, adapter)

	// Sort paths by depth (deepest first) for safe recursive directory renames
	// Only needed when renaming directories to avoid parent path invalidation
	if cfg.Directories {
		pathsToRename = engine.SortPathsByDepth(pathsToRename)
	}

	planResult := engine.Plan(pathsToRename)

	if !cfg.SkipHistory {
		store, err := history.NewGlobalStore(adapter)
		if err != nil {
			log.Warn("history disabled: %v\n", err)
		} else {
			command := strings.Join(os.Args, " ")

			_, err = store.Save(cfg.Path, history.Entry{
				Timestamp:  time.Now(),
				Command:    command,
				Version:    version.Version,
				Config:     cfg,
				Operations: mapEngineOperationToHistory(planResult.Operations),
				Skipped:    mapEngineSkippedFilesToHistory(planResult.Skipped),
				Collisions: mapEngineCollosionToHistory(planResult.Collisions),
			})

			if err != nil {
				log.Warn("could not save history: %v\n", err)
			}
		}
	}

	if len(planResult.Operations) == 0 {
		log.Info("\n✓ No files to rename\n")
		return nil
	}

	log.Debug("Processing %d file(s)...\n", len(planResult.Operations))

	err = fs.Apply(mapEngineToFS(planResult.Operations), cfg.DryRun)
	if err != nil {
		return fmt.Errorf("rename operation failed: %w", err)
	}

	printResults(planResult, cfg.DryRun)

	return nil
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

	log.Info("\n")
}

func mapEngineToFS(ops []engine.RenameOp) []fs.RenameOp {
	return common.MapSlice(ops, func(e engine.RenameOp) fs.RenameOp {
		return fs.RenameOp{
			OldPath: e.OldPath,
			NewPath: e.NewPath,
		}
	})
}

func mapEngineOperationToHistory(ops []engine.RenameOp) []history.Operation {
	return common.MapSlice(ops, func(e engine.RenameOp) history.Operation {
		return history.Operation{
			Old: e.OldPath,
			New: e.NewPath,
		}
	})
}

func mapEngineSkippedFilesToHistory(ops []engine.SkippedFile) []history.Skipped {
	return common.MapSlice(ops, func(e engine.SkippedFile) history.Skipped {
		return history.Skipped{
			Path:   e.Path,
			Reason: e.Reason,
		}
	})
}

func mapEngineCollosionToHistory(ops []engine.Collision) []history.Collision {
	return common.MapSlice(ops, func(e engine.Collision) history.Collision {
		return history.Collision{
			Source1: e.Source1,
			Source2: e.Source2,
			Target:  e.Target,
		}
	})
}
