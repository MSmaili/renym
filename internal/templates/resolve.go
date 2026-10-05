package templates

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var ErrAmbiguous = errors.New("ambiguous template name")

type Reference struct {
	Name      string
	Path      string
	Format    string
	Ambiguous bool
}

type Catalog struct {
	Directory string
	Templates []Reference
}

type locations struct {
	goos   string
	getenv func(string) string
	home   func() (string, error)
	config func() (string, error)
}

func systemLocations() locations {
	return locations{runtime.GOOS, os.Getenv, os.UserHomeDir, os.UserConfigDir}
}

// Directory returns the named-template storage path without creating it.
func Directory() (string, error) { return systemLocations().directory() }

func (l locations) directory() (string, error) {
	var base string
	var err error
	if l.goos == "windows" {
		base, err = l.config()
	} else {
		base = l.getenv("XDG_CONFIG_HOME")
		if !filepath.IsAbs(base) {
			base, err = l.home()
			if err == nil && filepath.IsAbs(base) {
				base = filepath.Join(base, ".config")
			}
		}
	}
	if err != nil {
		return "", fmt.Errorf("template directory: %w", err)
	}
	if !filepath.IsAbs(base) {
		return "", errors.New("template directory: home/config directory must be absolute")
	}
	return filepath.Join(base, "renym", "templates"), nil
}

func supportedExtension(ext string) bool { return ext == ".toml" || ext == ".yaml" || ext == ".yml" }

// Resolve treats separators/known suffixes as explicit paths. Bare names search
// only configured storage, never the working directory or parent projects.
func Resolve(reference string) (string, error) { return systemLocations().resolve(reference) }

func (l locations) resolve(reference string) (string, error) {
	if strings.ContainsAny(reference, `/\`) || supportedExtension(strings.ToLower(filepath.Ext(reference))) {
		return filepath.Abs(reference)
	}
	if !identifier.MatchString(reference) {
		return "", fmt.Errorf("invalid template name %q: expected a 1-64 byte ASCII name starting with a letter/digit", reference)
	}
	// Stream the directory and retain only this name's candidates; loading one
	// template need not allocate/sort a complete catalog.
	catalog, err := l.catalog(reference)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, candidate := range catalog.Templates {
		if candidate.Name == reference {
			matches = append(matches, candidate.Path)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("named template %q not found in %q: %w", reference, catalog.Directory, os.ErrNotExist)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("%w %q: %s; select an explicit file path", ErrAmbiguous, reference, strings.Join(matches, ", "))
	}
	return matches[0], nil
}

// List enumerates eligible filenames and ambiguity, not template contents.
// Missing storage is an empty catalog; listing never creates state/directories.
func List() (Catalog, error) { return systemLocations().list() }

func (l locations) list() (Catalog, error) { return l.catalog("") }

func (l locations) catalog(onlyName string) (Catalog, error) {
	directory, err := l.directory()
	if err != nil {
		return Catalog{}, err
	}
	catalog := Catalog{Directory: directory}
	info, err := os.Stat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return catalog, nil
	}
	if err != nil {
		return catalog, fmt.Errorf("list templates: %w", err)
	}
	if !info.IsDir() {
		return catalog, fmt.Errorf("template storage %q is not a directory", directory)
	}
	f, err := os.Open(directory)
	if err != nil {
		return catalog, fmt.Errorf("list templates: %w", err)
	}
	defer f.Close()
	counts := make(map[string]int)
	for {
		entries, err := f.ReadDir(128)
		if err != nil && !errors.Is(err, io.EOF) {
			return catalog, fmt.Errorf("list templates: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !entry.Type().IsRegular() && entry.Type()&os.ModeSymlink == 0 {
				continue
			}
			ext := filepath.Ext(entry.Name())
			name := strings.TrimSuffix(entry.Name(), ext)
			if onlyName != "" && name != onlyName {
				continue
			}
			format := strings.ToLower(ext)
			if !supportedExtension(format) || !identifier.MatchString(name) || supportedExtension(strings.ToLower(filepath.Ext(name))) {
				continue
			}
			catalog.Templates = append(catalog.Templates, Reference{Name: name, Path: filepath.Join(directory, entry.Name()), Format: format})
			counts[name]++
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	for i := range catalog.Templates {
		catalog.Templates[i].Ambiguous = counts[catalog.Templates[i].Name] > 1
	}
	sort.Slice(catalog.Templates, func(i, j int) bool {
		a, b := catalog.Templates[i], catalog.Templates[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Path < b.Path
	})
	return catalog, nil
}
