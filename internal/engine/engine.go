package engine

import (
	"path/filepath"
	"strings"
)

// TODO: here we should do all the planning for rename
// take care of file collision
// take care of skiped files
// take care of mods
// take care of spliting words

type RenameOp struct {
	OldPath string
	NewPath string
}
type FileSystemAdapter interface {
	IsCaseSensitive() bool
	SanitizeName(name string) string
}

type Engine struct {
	adapter FileSystemAdapter
	mode    RenameMode
}

func NewEngine(mode RenameMode, adapter FileSystemAdapter) *Engine {
	return &Engine{
		mode:    mode,
		adapter: adapter,
	}
}

func (e *Engine) Plan(paths []string) []RenameOp {

	renameOp := []RenameOp{}
	for _, path := range paths {

		dir := filepath.Dir(path)
		oldName := filepath.Base(path)
		ext := filepath.Ext(oldName)
		nameWithouExt := strings.TrimSuffix(oldName, ext)

		nameWithouExt = e.mode.Transform(nameWithouExt)

		newNameWithSuffic := nameWithouExt + ext

		newName := filepath.Join(dir, newNameWithSuffic)
		renameOp = append(renameOp, RenameOp{OldPath: path, NewPath: newName})
	}
	return renameOp
}

func Lower(name string) string {
	return strings.ToLower(name)
}
