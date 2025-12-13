package walker

import (
	"io/fs"
	"os"
	"path/filepath"
)

type Config struct {
	Path            string
	Recursive       bool
	Files           bool
	Directories     bool
	Ignore          []string
	NoDefaultIgnore bool
}

func isFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}

func Walk(cfg Config) ([]string, error) {
	isFile, err := isFile(cfg.Path)
	if err != nil {
		return nil, err
	}
	if isFile {
		if cfg.Files {
			return []string{cfg.Path}, nil
		}
		return []string{}, nil
	}

	paths := make([]string, 0, 100)

	ignorePatterns := cfg.Ignore
	if !cfg.NoDefaultIgnore {
		ignorePatterns = append(DefaultIgnorePatterns, cfg.Ignore...)
	}
	ignorePatterns = append(ignorePatterns, ".rnm-history")

	err = filepath.WalkDir(cfg.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == cfg.Path {
			return nil
		}

		if d.IsDir() && !cfg.Recursive && path != cfg.Path {
			return fs.SkipDir
		}

		name := d.Name()
		for _, pattern := range ignorePatterns {
			matched, err := filepath.Match(pattern, name)
			if err != nil {
				continue
			}
			if matched {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}

		if d.IsDir() && cfg.Directories {
			paths = append(paths, path)
		} else if !d.IsDir() && cfg.Files {
			paths = append(paths, path)
		}

		return nil
	})

	return paths, err
}
