package main

import (
	"github.com/MSmaili/rnm/internal/common"
	"github.com/MSmaili/rnm/internal/engine"
	"github.com/MSmaili/rnm/internal/fs"
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
)

type Config struct {
	Path            string
	Mode            string
	Recursive       bool
	Directories     bool
	Files           bool
	Ignore          []string
	NoDefaultIgnore bool
}

func init() {
	// Path flags
	rootCmd.Flags().StringVarP(&path, "path", "p", ".", "Path to directory or file")

	// Traversal flags
	rootCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively rename in subdirectories")

	// dirs flags
	rootCmd.Flags().BoolVarP(&directories, "directories", "d", false, "Include directories in rename")
	rootCmd.Flags().BoolVarP(&dirsOnly, "dirs-only", "D", false, "Rename only directories (not files)")

	// Filter flags
	rootCmd.Flags().StringSliceVar(&ignore, "ignore", nil, "Glob pattern to ignore (can be specified multiple times)")
	rootCmd.Flags().BoolVar(&noDefaultIgnore, "no-default-ignore", false, "Disable default ignore patterns (.git, .svn, .hg)")

	// Modes  flags
	rootCmd.Flags().StringVarP(&mode, "mode", "m", "lower", "Rename mode: upper, lower, pascal, camel, snake, kebab, title")
	rootCmd.RegisterFlagCompletionFunc("mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"upper", "lower", "pascal", "camel", "snake", "kebab", "title", "expr"}, cobra.ShellCompDirectiveNoFileComp
	})
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
	renameOp := engine.Plan(pathsToRename)

	return fs.Apply(mapEngineToFS(renameOp))
}

func mapEngineToFS(ops []engine.RenameOp) []fs.RenameOp {
	return common.MapSlice(ops, func(e engine.RenameOp) fs.RenameOp {
		return fs.RenameOp{
			OldPath: e.OldPath,
			NewPath: e.NewPath,
		}
	})
}
