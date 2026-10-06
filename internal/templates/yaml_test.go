package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/MSmaili/renym/internal/templates/render"
)

const yamlPreset = `version: 1
name: Screenshots
selection:
  ignore: ["*.tmp"]
rules:
  - id: screenshots
    match:
      glob: ["Screenshot*", "Screen Shot*"]
      extensions: [".png"]
    rename:
      mode: snake
`

func TestYAMLTOMLSpecAndRendererParity(t *testing.T) {
	pairs := []struct{ name, toml, yaml string }{
		{"screenshots", preset, yamlPreset},
		{"defaults", "version=1\n[[rules]]\nid='all'\n[rules.rename]\nmode='snake'\n", "version: 1\nrules:\n  - id: all\n    rename: {mode: snake}\n"},
		{"recursive filenames", `version=1
name='Indexed files'
[selection]
kind='both'
recursive=true
ignore=[]
no_default_ignore=true
[[rules]]
id='all'
[rules.rename]
filename='${file.stem | snake}_${index | pad(3)}${file.ext | lower}'
`, `version: 1
name: Indexed files
selection:
  kind: both
  recursive: true
  ignore: []
  no_default_ignore: true
rules:
  - id: all
    rename:
      filename: '${file.stem | snake}_${index | pad(3)}${file.ext | lower}'
`},
	}
	for _, name := range []string{"screenshots", "screenshot-dates", "organization-preview"} {
		toml, err := os.ReadFile(filepath.Join("..", "..", "examples", "templates", name+".toml"))
		if err != nil {
			t.Fatal(err)
		}
		yaml, err := os.ReadFile(filepath.Join("..", "..", "examples", "templates", name+".yaml"))
		if err != nil {
			t.Fatal(err)
		}
		pairs = append(pairs, struct{ name, toml, yaml string }{"example " + name, string(toml), string(yaml)})
	}
	modified, size := time.Date(2026, 10, 5, 12, 13, 14, 0, time.UTC), int64(123)
	for _, pair := range pairs {
		t.Run(pair.name, func(t *testing.T) {
			a, err := Parse([]byte(pair.toml))
			if err != nil {
				t.Fatal(err)
			}
			b, err := ParseYAML([]byte(pair.yaml))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(a.Snapshot(), b.Snapshot()) {
				t.Fatalf("normalized specs differ:\n%+v\n%+v", a.Snapshot(), b.Snapshot())
			}
			for _, ctx := range []render.Context{{Name: "Screenshot One.PNG", Modified: &modified, Size: &size, Index: 7}, {Name: "project.v1", Directory: true, Modified: &modified, Index: 2}, {Name: ".env", Index: 1}, {Name: "archive.tar.gz", Index: 3}} {
				da, ma := a.Match(ctx.Name, ctx.Directory)
				db, mb := b.Match(ctx.Name, ctx.Directory)
				if ma != mb || da.RuleID != db.RuleID || da.Mode != db.Mode || da.Dependencies() != db.Dependencies() {
					t.Fatal("format changes matching/dependencies")
				}
				if da.Moves() != db.Moves() || da.MoveRoot() != db.MoveRoot() {
					t.Fatal("format changes move action")
				}
				if ma && da.Moves() {
					a, ea := da.RenderDirectory(ctx)
					b, eb := db.RenderDirectory(ctx)
					if a != b || fmt.Sprint(ea) != fmt.Sprint(eb) {
						t.Fatal("format changes directory rendering")
					}
				}
				if ma && da.RendersFilename() {
					na, ea := da.Render(ctx)
					nb, eb := db.Render(ctx)
					if na != nb || fmt.Sprint(ea) != fmt.Sprint(eb) {
						t.Fatalf("format changes evaluation: %q %v / %q %v", na, ea, nb, eb)
					}
				}
			}
		})
	}
}

func TestYAMLStrictSubset(t *testing.T) {
	valid := "version: 1\nrules:\n  - id: all\n    rename: {mode: snake}\n"
	for name, source := range map[string]string{
		"empty": "", "invalid UTF8": string([]byte{0xff}), "empty document": "---\n", "root scalar": "hello", "root sequence": "[1, 2]",
		"second document": valid + "---\nversion: 1\n", "empty second document": valid + "---\n", "trailing malformed": valid + "---\n[",
		"duplicate root key": "version: 1\n" + valid, "duplicate nested key": strings.Replace(valid, "{mode: snake}", "{mode: snake, mode: kebab}", 1),
		"unknown key": "extra: true\n" + valid, "case alias": strings.Replace(valid, "version:", "Version:", 1),
		"numeric string": strings.Replace(valid, "id: all", "id: 123", 1), "timestamp string": strings.Replace(valid, "id: all", "id: 2026-10-05", 1),
		"null mode": strings.Replace(valid, "mode: snake", "mode: null", 1), "empty mode": strings.Replace(valid, "mode: snake", "mode:", 1),
		"null list": "selection: {ignore: null}\n" + valid, "scalar list": "selection: {ignore: '*.tmp'}\n" + valid,
		"legacy boolean": "selection: {recursive: yes}\n" + valid, "uppercase boolean": "selection: {recursive: True}\n" + valid,
		"quoted boolean": "selection: {recursive: 'false'}\n" + valid, "numeric boolean": "selection: {recursive: 0}\n" + valid,
		"float version": strings.Replace(valid, "version: 1", "version: 1.0", 1), "quoted version": strings.Replace(valid, "version: 1", "version: '1'", 1),
		"octal version": strings.Replace(valid, "version: 1", "version: 01", 1), "hex version": strings.Replace(valid, "version: 1", "version: 0x1", 1),
		"signed version": strings.Replace(valid, "version: 1", "version: +1", 1), "exponent version": strings.Replace(valid, "version: 1", "version: 1e0", 1),
		"overflow version":         strings.Replace(valid, "version: 1", "version: 9223372036854775808", 1),
		"anchor":                   strings.Replace(valid, "mode: snake", "mode: &mode snake", 1),
		"alias":                    "name: &label all\n" + strings.Replace(valid, "id: all", "id: *label", 1),
		"merge":                    "selection: {<<: {recursive: true}}\n" + valid,
		"custom tag":               strings.Replace(valid, "mode: snake", "mode: !command snake", 1),
		"complex key":              "? [a, b]\n: true\n" + valid,
		"both actions":             strings.Replace(valid, "{mode: snake}", "{mode: snake, filename: '${file.name}'}", 1),
		"empty mode plus filename": strings.Replace(valid, "{mode: snake}", "{mode: '', filename: '${file.name}'}", 1),
		"no action":                strings.Replace(valid, "{mode: snake}", "{}", 1),
		"unsupported source":       "source: {path: '~/Downloads'}\n" + valid,
		"unknown move field":       strings.Replace(valid, "    rename:", "    move: {root: '~/output', overwrite: true}\n    rename:", 1),
		"empty glob":               valid + "    match: {glob: []}\n", "bad glob": valid + "    match: {glob: ['[']}\n",
		"bad filename": strings.Replace(valid, "{mode: snake}", "{filename: '${env.HOME}'}", 1),
		"oversized":    strings.Repeat("#", MaxBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if compiled, err := ParseYAML([]byte(source)); err == nil || compiled != nil {
				t.Fatalf("invalid YAML accepted: %q", source)
			}
		})
	}
	_, err := ParseYAML([]byte("version: 1\nselection: {recursive: yes}\nrules: []\n"))
	if err == nil || !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "template.selection.recursive") {
		t.Fatalf("missing YAML location/field: %v", err)
	}
}

func TestYAMLStringSpellingAndLimits(t *testing.T) {
	// Legacy yes/no/on/off are ordinary strings when a string field is requested.
	for _, id := range []string{"yes", "no", "on", "off", "'123'"} {
		if _, err := ParseYAML([]byte("version: 1\nrules:\n  - id: " + id + "\n    rename: {mode: snake}\n")); err != nil {
			t.Fatalf("string was coerced: %s, %v", id, err)
		}
	}
	base := "version: 1\nrules:\n  - id: all\n    rename:\n      filename: |\n        ${file.stem}\n"
	compiled, err := ParseYAML([]byte(base))
	if err != nil {
		t.Fatal(err)
	}
	decision, _ := compiled.Match("Some File.txt", false)
	if name, err := decision.Render(render.Context{Name: "Some File.txt"}); err == nil || name != "" {
		t.Fatal("YAML block scalar trailing newline silently repaired")
	}
	var source strings.Builder
	source.WriteString("version: 1\nrules:\n")
	for i := range MaxRules + 1 {
		fmt.Fprintf(&source, "  - id: r%d\n    rename: {mode: snake}\n", i)
	}
	if _, err := ParseYAML([]byte(source.String())); err == nil {
		t.Fatal("rule cap not shared")
	}
	// Structural limits are enforced before typed decode, even on synthetic nodes.
	budget := 0
	node := yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "test"}
	if err := checkYAML(&node, "string", "test", 1, &budget); err == nil {
		t.Fatal("node budget ignored")
	}
	budget = MaxYAMLNodes
	if err := checkYAML(&node, "string", "test", MaxYAMLDepth+1, &budget); err == nil {
		t.Fatal("depth budget ignored")
	}
}

func FuzzParseYAML(f *testing.F) {
	for _, source := range []string{yamlPreset, "version: 1\nrules:\n - id: all\n   rename: {filename: '${file.stem | snake}${file.ext}'}\n", "---\n---\n", "name: &x [*x]", "version: 01", ""} {
		f.Add([]byte(source))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		compiled, err := ParseYAML(data)
		if err != nil {
			if compiled != nil {
				t.Fatal("invalid YAML returned a usable template")
			}
			return
		}
		spec := compiled.Snapshot()
		if spec.Version != Version || len(spec.Rules) == 0 || len(spec.Rules) > MaxRules {
			t.Fatalf("unvalidated YAML: %+v", spec)
		}
		decision, matched := compiled.Match("Screenshot One.PNG", false)
		if matched && decision.RendersFilename() {
			ctx := render.Context{Name: "Screenshot One.PNG", Index: 1}
			first, a := decision.Render(ctx)
			second, b := decision.Render(ctx)
			if first != second || fmt.Sprint(a) != fmt.Sprint(b) || a != nil && first != "" {
				t.Fatal("nondeterministic or partial render")
			}
		}
	})
}
