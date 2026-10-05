package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const preset = `version = 1
name = "Screenshots"
[selection]
ignore = ["*.tmp"]
[[rules]]
id = "screenshots"
[rules.match]
glob = ["Screenshot*", "Screen Shot*"]
extensions = [".png"]
[rules.rename]
mode = "snake"
`

func TestParseDefaultsAndSnapshotIsolation(t *testing.T) {
	compiled, err := Parse([]byte(preset))
	if err != nil {
		t.Fatal(err)
	}
	spec := compiled.Snapshot()
	if spec.Version != Version || spec.Name != "Screenshots" || spec.Selection.Kind != "files" || spec.Selection.Recursive || spec.Selection.NoDefaultIgnore {
		t.Fatalf("wrong defaults: %+v", spec)
	}
	spec.Selection.Ignore[0] = "*"
	spec.Rules[0].Match.Glob[0] = "*"
	spec.Rules[0].Match.Extensions[0] = ".txt"
	spec.Rules[0].Rename.Mode = "upper"
	selection := compiled.Selection()
	selection.Ignore[0] = "*"
	fresh := compiled.Snapshot()
	if fresh.Selection.Ignore[0] != "*.tmp" || fresh.Rules[0].Match.Glob[0] != "Screenshot*" || fresh.Rules[0].Match.Extensions[0] != ".png" || fresh.Rules[0].Rename.Mode != "snake" {
		t.Fatal("presentation mutated compiled rules")
	}
}

func TestInvalidPresets(t *testing.T) {
	valid := "version = 1\n[[rules]]\nid = 'all'\n[rules.rename]\nmode = 'snake'\n"
	for name, source := range map[string]string{
		"case alias version":       strings.Replace(valid, "version = 1", "Version = 1", 1),
		"folded duplicate version": "Version = 1\n" + valid,
		"case alias rename":        strings.Replace(valid, "mode = 'snake'", "Mode = 'snake'", 1),
		"case alias ID":            strings.Replace(valid, "id = 'all'", "ID = 'all'", 1),
		"case alias table":         strings.Replace(valid, "[rules.rename]", "[rules.Rename]", 1),
		"missing version":          strings.Replace(valid, "version = 1\n", "", 1),
		"unknown version":          strings.Replace(valid, "version = 1", "version = 2", 1),
		"unknown mode":             strings.Replace(valid, "'snake'", "'run-shell'", 1),
		"missing mode":             strings.Replace(valid, "mode = 'snake'", "", 1),
		"unknown root field":       "extra = true\n" + valid,
		"unknown empty table":      valid + "\n[future]\n",
		"both rename actions":      valid + "filename = '${file.stem}'\n",
		"future move":              valid + "move = '/tmp'\n",
		"duplicate key":            "version = 1\n" + valid,
		"duplicate ID":             valid + "\n[[rules]]\nid = 'all'\n[rules.rename]\nmode = 'kebab'\n",
		"missing ID":               strings.Replace(valid, "id = 'all'\n", "", 1),
		"invalid ID":               strings.Replace(valid, "id = 'all'", "id = 'with spaces'", 1),
		"empty label":              "name = ''\n" + valid,
		"label whitespace":         "name = ' trailing '\n" + valid,
		"no rules":                 "version = 1\n",
		"bad kind":                 "version = 1\n[selection]\nkind = 'fish'\n" + strings.TrimPrefix(valid, "version = 1\n"),
		"empty kind":               "version = 1\n[selection]\nkind = ''\n" + strings.TrimPrefix(valid, "version = 1\n"),
		"wrong bool":               "version = 1\n[selection]\nrecursive = 'false'\n" + strings.TrimPrefix(valid, "version = 1\n"),
		"empty glob":               valid + "[rules.match]\nglob = []\n",
		"bad glob":                 valid + "[rules.match]\nglob = ['[']\n",
		"bad ignore":               "version = 1\n[selection]\nignore = ['[']\n" + strings.TrimPrefix(valid, "version = 1\n"),
		"recursive glob":           valid + "[rules.match]\nglob = ['**.png']\n",
		"path glob":                valid + "[rules.match]\nglob = ['sub/*.png']\n",
		"wrong glob type":          valid + "[rules.match]\nglob = '*.png'\n",
		"empty extension":          valid + "[rules.match]\nextensions = []\n",
		"uppercase extension":      valid + "[rules.match]\nextensions = ['.PNG']\n",
		"compound extension":       valid + "[rules.match]\nextensions = ['.tar.gz']\n",
		"missing extension dot":    valid + "[rules.match]\nextensions = ['png']\n",
		"extension wildcard":       valid + "[rules.match]\nextensions = ['.*']\n",
		"extension whitespace":     valid + "[rules.match]\nextensions = ['.p ng']\n",
		"unknown match field":      valid + "[rules.match]\nregex = '.*'\n",
		"wrong version type":       strings.Replace(valid, "version = 1", "version = '1'", 1),
		"oversized source":         strings.Repeat("#", MaxBytes+1),
		"invalid UTF8":             string([]byte{0xff}),
	} {
		t.Run(name, func(t *testing.T) {
			if compiled, err := Parse([]byte(source)); err == nil || compiled != nil {
				t.Fatalf("invalid preset accepted: %q", source)
			}
		})
	}
}

func TestFirstMatchAndOriginalBasenameSemantics(t *testing.T) {
	source := preset + `[[rules]]
id = "fallback"
[rules.match]
glob = ["*.png", "project*"]
[rules.rename]
mode = "kebab"
`
	compiled, err := Parse([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		directory bool
		id        string
	}{
		{"Screenshot One.png", false, "screenshots"},
		{"Screen Shot Two.PNG", false, "screenshots"},
		{"Screenshot One.txt", false, ""},
		{"screenshot One.png", false, "fallback"},
		{"Other.png", false, "fallback"},
		{"Screenshot One.png", true, "fallback"},
		{"project.v1", true, "fallback"},
		{".env", false, ""},
	} {
		t.Run(test.name+fmt.Sprint(test.directory), func(t *testing.T) {
			decision, matched := compiled.Match(test.name, test.directory)
			if matched != (test.id != "") || decision.RuleID != test.id {
				t.Fatalf("%q: got %+v, %t; want %s", test.name, decision, matched, test.id)
			}
		})
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				decision, _ := compiled.Match("Screenshot One.png", false)
				if decision.RuleID != "screenshots" {
					t.Errorf("nondeterministic matcher: %+v", decision)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestExtensionMatchingUsesLastExtensionAndDotfilePolicy(t *testing.T) {
	compiled, err := Parse([]byte("version=1\n[[rules]]\nid='extensions'\n[rules.match]\nextensions=['.gz', '.env']\n[rules.rename]\nmode='snake'\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name            string
		directory, want bool
	}{
		{"archive.tar.gz", false, true}, {"archive.GZ", false, true}, {".env", false, false}, {".local.env", false, true}, {"project.gz", true, false},
	} {
		_, got := compiled.Match(test.name, test.directory)
		if got != test.want {
			t.Errorf("%s directory=%t: got %t", test.name, test.directory, got)
		}
	}
}

func TestLoadExplicitFileAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "preset.toml")
	if err := os.WriteFile(filename, []byte(preset), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filename); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "directory.toml")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(directory); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("non-file input accepted: %v", err)
	}
	for _, path := range []string{filepath.Join(root, "absent.toml"), "preset", filepath.Join(root, "preset.yaml"), root + ".toml"} {
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), strconv.Quote(path)) {
			t.Fatalf("missing path diagnostic: %s, %v", path, err)
		}
	}
	if err := os.WriteFile(filename, []byte("version = 'oops'"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(filename)
	if err == nil || !strings.Contains(err.Error(), strconv.Quote(filename)) || !strings.Contains(err.Error(), "1|") {
		t.Fatalf("missing parser location: %v", err)
	}
	if err := os.WriteFile(filename, []byte(strings.Repeat("#", MaxBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filename); err == nil {
		t.Fatal("oversized file accepted")
	}
}

func TestLimits(t *testing.T) {
	var source strings.Builder
	source.WriteString("version=1\n")
	for i := range MaxRules + 1 {
		fmt.Fprintf(&source, "[[rules]]\nid='rule-%d'\n[rules.rename]\nmode='snake'\n", i)
	}
	if _, err := Parse([]byte(source.String())); err == nil {
		t.Fatal("too many rules accepted")
	}
	source.Reset()
	source.WriteString("version=1\n[[rules]]\nid='all'\n[rules.rename]\nmode='snake'\n[rules.match]\nglob=[")
	for range MaxPatterns + 1 {
		source.WriteString("'*',")
	}
	source.WriteString("]\n")
	if _, err := Parse([]byte(source.String())); err == nil {
		t.Fatal("too many patterns accepted")
	}
	base := "version=1\n[[rules]]\nid='all'\n[rules.rename]\nmode='snake'\n"
	if _, err := Parse([]byte(base + "#" + strings.Repeat("x", MaxBytes-len(base)-1))); err != nil {
		t.Fatalf("exact byte limit rejected: %v", err)
	}
	if _, err := Parse([]byte(base + "[rules.match]\nglob=['" + strings.Repeat("x", MaxPatternBytes+1) + "']\n")); err == nil {
		t.Fatal("oversized glob accepted")
	}
	if _, err := Parse([]byte("name='" + strings.Repeat("x", 129) + "'\n" + base)); err == nil {
		t.Fatal("oversized label accepted")
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(preset))
	f.Add([]byte("version=1\n[[rules]]\nid='all'\n[rules.rename]\nfilename='${file.stem | snake}${file.ext}'\n"))
	f.Add([]byte("version=1\n"))
	f.Add([]byte("[rules]\nfilename='${shell}'"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		compiled, err := Parse(data)
		if err != nil {
			if compiled != nil {
				t.Fatal("failed parse returned a usable template")
			}
			return
		}
		spec := compiled.Snapshot()
		if spec.Version != Version || len(spec.Rules) == 0 || len(spec.Rules) > MaxRules {
			t.Fatalf("unvalidated spec: %+v", spec)
		}
		one, matched := compiled.Match("Screenshot One.png", false)
		two, matchedAgain := compiled.Match("Screenshot One.png", false)
		if one != two || matched != matchedAgain {
			t.Fatal("matching is not deterministic")
		}
	})
}
