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
			fmt.Println("❗ The --mode flag requires a value.")
			fmt.Println("Available modes: upper, lower, pascal, camel, snake, kebab, title")
			fmt.Println("\nRun rnm --help for more info")
			os.Exit(1)
		}

		return err
	})
}

func validateFlags(cmd *cobra.Command, args []string) error {
	if showVersion {
		fmt.Printf("rnm version %s\n", version.Version)
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
		command := strings.Join(os.Args, " ")

		err = history.Save(cfg.Path, history.Entry{
			Path:       cfg.Path,
			Timestamp:  time.Now(),
			Command:    command,
			Version:    version.Version,
			Config:     cfg,
			Operations: mapEngineOperationToHistory(planResult.Operations),
			Skipped:    mapEngineSkippedFilesToHistory(planResult.Skipped),
			Collisions: mapEngineCollosionToHistory(planResult.Collisions),
		})

		if err != nil {
			return fmt.Errorf("we could not save history, you can use skip-history")
		}
	}

	if len(planResult.Operations) == 0 {
		fmt.Println("\n✓ No files to rename")
		return nil
	}

	fmt.Printf("\nProcessing %d file(s)...\n", len(planResult.Operations))

	err = fs.Apply(mapEngineToFS(planResult.Operations), cfg.DryRun)
	if err != nil {
		return fmt.Errorf("rename operation failed: %w", err)
	}

	printResults(planResult, cfg.DryRun)

	return nil
}

func printResults(result engine.PlanResult, dryRun bool) {
	// Success header
	fmt.Println("\n" + strings.Repeat("=", 60))
	if dryRun {
		fmt.Println("  DRY RUN - No files were actually renamed")
	} else {
		fmt.Println("  ✓ COMPLETED SUCCESSFULLY")
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("  Files renamed:   %d\n", len(result.Operations))
	}

	// Show warnings if any
	if len(result.Skipped) > 0 {
		fmt.Printf("  Files skipped:   %d\n", len(result.Skipped))
	}
	if len(result.Collisions) > 0 {
		fmt.Printf("  Collisions:      %d\n", len(result.Collisions))
	}
	fmt.Println(strings.Repeat("=", 60))

	// Show collision details
	if len(result.Collisions) > 0 {
		fmt.Println("\n⚠ COLLISIONS:")
		fmt.Println(strings.Repeat("-", 60))
		for i, collision := range result.Collisions {
			fmt.Printf("  %d. Multiple files trying to rename to:\n", i+1)
			fmt.Printf("     → %s\n", filepath.Base(collision.Target))
			fmt.Printf("     Sources: %s, %s\n", filepath.Base(collision.Source1), filepath.Base(collision.Source2))
			if i < len(result.Collisions)-1 {
				fmt.Println()
			}
		}
		fmt.Println(strings.Repeat("-", 60))
	}

	fmt.Println()
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
