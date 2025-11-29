package main

import (
	"github.com/MSmaili/rnm/internal/common"
	"github.com/MSmaili/rnm/internal/engine"
	"github.com/MSmaili/rnm/internal/fs"
	"github.com/MSmaili/rnm/internal/walker"
	"github.com/spf13/cobra"
)

var (
	mode string
	path string
)

type Config struct {
	Path string
	Mode string
}

func init() {
	rootCmd.Flags().StringVarP(&path, "path", "p", ".", "Path to directory or file")
	rootCmd.Flags().StringVarP(&mode, "mode", "m", "lower", "Rename mode: upper, lower, pascal, camel, snake, kebab, title")
}

func runRename(cmd *cobra.Command, args []string) error {
	cfg := Config{
		Path: path,
		Mode: mode,
	}

	adapter := fs.NewAdapter()

	pathsToRename, err := walker.Walk(walker.Config{
		Path:        cfg.Path,
		Recursive:   false,
		Directories: false,
		Files:       true,
		Ignore:      []string{},
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
