package walker

import (
	"io/fs"
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

func Walk(cfg Config) ([]string, error) {
	paths := make([]string, 0, 100)

	ignorePatterns := cfg.Ignore
	if !cfg.NoDefaultIgnore {
		ignorePatterns = append(DefaultIgnorePatterns, cfg.Ignore...)
	}

	err := filepath.WalkDir(cfg.Path, func(path string, d fs.DirEntry, err error) error {
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
