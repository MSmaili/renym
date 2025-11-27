package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

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

	fmt.Println("calling", cfg.Path)
	fmt.Println("mode", cfg.Mode)

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

	renameOp := []fs.RenameOp{}
	for _, path := range pathsToRename {

		dir := filepath.Dir(path)
		oldName := filepath.Base(path)
		ext := filepath.Ext(oldName)
		nameWithouExt := strings.TrimSuffix(oldName, ext)

		if cfg.Mode == "pascal" {
			nameWithouExt = Pascal(nameWithouExt)
			nameWithouExt = adapter.SanitizeName(nameWithouExt)
		} else {
			nameWithouExt = Lower(nameWithouExt)
			nameWithouExt = adapter.SanitizeName(nameWithouExt)
		}

		newNameWithSuffic := nameWithouExt + ext

		newName := filepath.Join(dir, newNameWithSuffic)
		renameOp = append(renameOp, fs.RenameOp{OldPath: path, NewPath: newName})
		fmt.Println(renameOp)
	}

	return fs.Apply(renameOp)
}

func Lower(name string) string {
	return strings.ToLower(name)
}

var wordRegex = regexp.MustCompile(`[A-Za-z0-9]+`)

func Pascal(name string) string {
	words := wordRegex.FindAllString(name, -1)
	for i := range words {
		words[i] = strings.Title(strings.ToLower(words[i]))
	}
	return strings.Join(words, "")
}
