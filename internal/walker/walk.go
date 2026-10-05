package walker

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

type Config struct {
	Path            string
	Recursive       bool
	Files           bool
	Directories     bool
	Ignore          []string
	NoDefaultIgnore bool
	PortableGlobs   bool
}

func isFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}

func Walk(cfg Config) ([]string, error) {
	return WalkContext(context.Background(), cfg)
}

func WalkContext(ctx context.Context, cfg Config) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	isFile, err := isFile(cfg.Path)
	if err != nil {
		return nil, err
	}
	if isFile {
		if cfg.Files {
			// Existing mode commands keep their explicit-file behavior. Presets
			// apply portable basename filters to a single file as well as batches.
			if cfg.PortableGlobs {
				patterns := cfg.Ignore
				if !cfg.NoDefaultIgnore {
					patterns = append(append([]string(nil), DefaultIgnorePatterns...), cfg.Ignore...)
				}
				for _, pattern := range patterns {
					if matched, _ := path.Match(pattern, filepath.Base(cfg.Path)); matched {
						return []string{}, nil
					}
				}
			}
			return []string{cfg.Path}, nil
		}
		return []string{}, nil
	}

	paths := make([]string, 0, 100)

	ignorePatterns := cfg.Ignore
	if !cfg.NoDefaultIgnore {
		ignorePatterns = append(append([]string(nil), DefaultIgnorePatterns...), cfg.Ignore...)
	}
	match := filepath.Match
	if cfg.PortableGlobs {
		match = path.Match
	}

	err = filepath.WalkDir(cfg.Path, func(path string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			return err
		}

		if path == cfg.Path {
			return nil
		}

		name := d.Name()
		for _, pattern := range ignorePatterns {
			matched, err := match(pattern, name)
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

		if d.IsDir() {
			if cfg.Directories {
				paths = append(paths, path)
			}
			if !cfg.Recursive {
				return fs.SkipDir
			}
		} else if cfg.Files {
			paths = append(paths, path)
		}

		return nil
	})

	return paths, err
}
