package engine

import (
	"path/filepath"
	"regexp"
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
	mode    string
}

func NewEngine(mode string, adapter FileSystemAdapter) *Engine {
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

		if e.mode == "pascal" {
			nameWithouExt = Pascal(nameWithouExt)
			nameWithouExt = e.adapter.SanitizeName(nameWithouExt)
		} else {
			nameWithouExt = Lower(nameWithouExt)
			nameWithouExt = e.adapter.SanitizeName(nameWithouExt)
		}

		newNameWithSuffic := nameWithouExt + ext

		newName := filepath.Join(dir, newNameWithSuffic)
		renameOp = append(renameOp, RenameOp{OldPath: path, NewPath: newName})
	}
	return renameOp
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
