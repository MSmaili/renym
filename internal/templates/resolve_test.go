package templates

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestTemplateDirectoryPolicy(t *testing.T) {
	base := t.TempDir()
	home, config, xdg := filepath.Join(base, "home"), filepath.Join(base, "native"), filepath.Join(base, "xdg")
	failure := errors.New("location unavailable")
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, value := range []string{"", "relative", xdg} {
			t.Run(goos+"/"+value, func(t *testing.T) {
				l := locations{goos: goos, getenv: func(string) string { return value }, home: func() (string, error) { return home, nil }, config: func() (string, error) { return config, nil }}
				want := filepath.Join(home, ".config", "renym", "templates")
				if value == xdg {
					want = filepath.Join(xdg, "renym", "templates")
				}
				if goos == "windows" {
					want = filepath.Join(config, "renym", "templates")
				}
				got, err := l.directory()
				if err != nil || got != want {
					t.Fatalf("directory = %q, %v; want %q", got, err, want)
				}
				if goos == "windows" {
					l.config = func() (string, error) { return "", failure }
				} else {
					l.home = func() (string, error) { return "", failure }
				}
				_, err = l.directory()
				if goos != "windows" && value == xdg {
					if err != nil {
						t.Fatal("absolute XDG unnecessarily depends on home", err)
					}
				} else if !errors.Is(err, failure) {
					t.Fatalf("lookup failure lost: %v", err)
				}
				l.home = func() (string, error) { return "relative", nil }
				l.config = func() (string, error) { return "relative", nil }
				if goos == "windows" || value != xdg {
					if _, err := l.directory(); err == nil {
						t.Fatal("relative home/config accepted")
					}
				}
			})
		}
	}
}

func isolatedTemplateLocations(t *testing.T) locations {
	t.Helper()
	base := t.TempDir()
	return locations{goos: "linux", getenv: func(string) string { return base }, home: func() (string, error) { t.Fatal("unexpected home lookup"); return "", nil }, config: func() (string, error) { t.Fatal("unexpected config lookup"); return "", nil }}
}

func TestMissingCatalogAndNamesNeverCreateDirectories(t *testing.T) {
	l := isolatedTemplateLocations(t)
	dir, _ := l.directory()
	catalog, err := l.list()
	if err != nil || catalog.Directory != dir || len(catalog.Templates) != 0 {
		t.Fatalf("missing catalog: %+v %v", catalog, err)
	}
	if _, err := l.resolve("missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing name: %v", err)
	}
	for _, name := range []string{"", ".", "..", " space", "a b", "日本語", strings.Repeat("a", 65), "$HOME"} {
		if _, err := l.resolve(name); err == nil {
			t.Fatalf("invalid name accepted: %q", name)
		}
	}
	if _, err := os.Stat(filepath.Dir(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lookup created storage: %v", err)
	}
}

func TestNamesNeverSearchWorkingDirectory(t *testing.T) {
	l := isolatedTemplateLocations(t)
	local := t.TempDir()
	if err := os.WriteFile(filepath.Join(local, "local.toml"), []byte(preset), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(local)
	if _, err := l.resolve("local"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("named lookup used local file: %v", err)
	}
	want := filepath.Join(local, "local.toml")
	if got, err := l.resolve("local.toml"); err != nil || got != want {
		t.Fatalf("explicit file not relative to cwd: %q %v", got, err)
	}
	// Explicit paths also work when config/home storage cannot be determined.
	l.getenv = func(string) string { return "" }
	l.home = func() (string, error) { return "", errors.New("no home") }
	if got, err := l.resolve("local.toml"); err != nil || got != want {
		t.Fatalf("explicit load unnecessarily requires storage: %q %v", got, err)
	}
}

func TestCatalogAndAmbiguousLookup(t *testing.T) {
	l := isolatedTemplateLocations(t)
	dir, _ := l.directory()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"shots.toml", "shots.yaml", "unique.YML", "z-last.toml", "invalid label.yaml", "ignored.txt", ".hidden.toml", "looks.toml.yaml", "a-first.yaml"} {
		// Listing describes origin paths, not validity. Compilation happens at load.
		if err := os.WriteFile(filepath.Join(dir, name), []byte("invalid"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "directory.yaml"), 0700); err != nil {
		t.Fatal(err)
	}
	catalog, err := l.list()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, ref := range catalog.Templates {
		names = append(names, ref.Name)
		if !filepath.IsAbs(ref.Path) || ref.Ambiguous != (ref.Name == "shots") {
			t.Fatalf("wrong origin/ambiguity: %+v", ref)
		}
	}
	if !reflect.DeepEqual(names, []string{"a-first", "shots", "shots", "unique", "z-last"}) {
		t.Fatalf("catalog: %v", names)
	}
	if _, err := l.resolve("shots"); !errors.Is(err, ErrAmbiguous) || !strings.Contains(err.Error(), "shots.toml") || !strings.Contains(err.Error(), "shots.yaml") {
		t.Fatalf("ambiguity hidden: %v", err)
	}
	if got, err := l.resolve("unique"); err != nil || got != filepath.Join(dir, "unique.YML") {
		t.Fatalf("named lookup: %s %v", got, err)
	}
	if _, err := l.resolve("Unique"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("names must match exactly: %v", err)
	}
	// Explicit selectors are never interpreted as configured names.
	for _, input := range []string{"shots.yaml", "shots.TOML", "./unique", filepath.Join(dir, "shots.yaml"), `folder\preset.yml`} {
		want, _ := filepath.Abs(input)
		got, err := l.resolve(input)
		if err != nil || got != want {
			t.Fatalf("explicit path changed: %q => %q %v", input, got, err)
		}
	}
	if err := os.Remove(filepath.Join(dir, "shots.yaml")); err != nil {
		t.Fatal(err)
	}
	if got, err := l.resolve("shots"); err != nil || got != filepath.Join(dir, "shots.toml") {
		t.Fatalf("remaining name not resolved: %q %v", got, err)
	}
}

func TestStorageIsFile(t *testing.T) {
	l := isolatedTemplateLocations(t)
	dir, _ := l.directory()
	if err := os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.list(); err == nil {
		t.Fatal("file storage treated as empty catalog")
	}
	if _, err := l.resolve("all"); err == nil {
		t.Fatal("file storage treated as valid")
	}
}

func TestLoadFormatsAndNamedOrigins(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("APPDATA", base)
	dir, err := Directory()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".toml", ".yaml", ".yml", ".YAML"} {
		path := filepath.Join(dir, "preset"+ext)
		data := preset
		if ext != ".toml" {
			data = yamlPreset
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		for _, ref := range []string{path, "preset"} {
			compiled, err := Load(ref)
			if err != nil || compiled.SourcePath() != path {
				t.Fatalf("origin: %q => %+v %v", ref, compiled, err)
			}
			if compiled.Snapshot().Name != "Screenshots" {
				t.Fatal("wrong format")
			}
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("version: '1'"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("bad"); err == nil || !strings.Contains(err.Error(), strconv.Quote(path)) || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("lost file/location: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("#", MaxBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("load byte cap: %v", err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("directory accepted as template")
	}
	if _, err := Load(filepath.Join(dir, "bad.json")); err == nil || !strings.Contains(err.Error(), "supported formats") {
		t.Fatalf("unsupported format: %v", err)
	}
}

func TestTemplateSymlinkAndDanglingLookup(t *testing.T) {
	l := isolatedTemplateLocations(t)
	dir, _ := l.directory()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target.yaml")
	if err := os.WriteFile(target, []byte(yamlPreset), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	compiled, err := Load(link)
	if err != nil || compiled.SourcePath() != link {
		t.Fatalf("explicit symlink load: %+v %v", compiled, err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if got, err := l.resolve("linked"); err != nil || got != link {
		t.Fatalf("listing should not load a dangling link: %q %v", got, err)
	}
	if _, err := Load(link); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dangling template loaded: %v", err)
	}
}
