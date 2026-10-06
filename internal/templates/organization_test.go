package templates

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MSmaili/renym/internal/templates/render"
)

func TestOrganizationFormatParityAndSnapshots(t *testing.T) {
	root := filepath.Join(t.TempDir(), "output")
	toml := fmt.Sprintf(`version=1
[[rules]]
id='images'
[rules.move]
root=%q
directory='screenshots/${file.modified | date("month")}'
[rules.rename]
filename='${file.stem | snake}${file.ext | lower}'
[[rules]]
id='fallback'
[rules.move]
root=%q
`, root, root)
	yaml := fmt.Sprintf(`version: 1
rules:
  - id: images
    move:
      root: %q
      directory: 'screenshots/${file.modified | date("month")}'
    rename:
      filename: '${file.stem | snake}${file.ext | lower}'
  - id: fallback
    move: {root: %q}
`, root, root)
	a, err := Parse([]byte(toml))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseYAML([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Snapshot(), b.Snapshot()) || !a.HasMoves() {
		t.Fatal("format-neutral move spec lost")
	}
	stamp := time.Date(2026, 10, 6, 1, 2, 3, 0, time.UTC)
	for _, compiled := range []*Compiled{a, b} {
		decision, ok := compiled.Match("Screenshot One.PNG", false)
		if !ok || !decision.Moves() || decision.MoveRoot() != root || !decision.Dependencies().Modified {
			t.Fatalf("move/dependency missing: %+v", decision)
		}
		directory, err := decision.RenderDirectory(render.Context{Name: "Screenshot One.PNG", Modified: &stamp})
		if err != nil || directory != "screenshots/2026-10" {
			t.Fatalf("directory: %q %v", directory, err)
		}
		snapshot := compiled.Snapshot()
		snapshot.Rules[0].Move.Root = "changed"
		*snapshot.Rules[0].Move.Directory = "changed"
		if compiled.Snapshot().Rules[0].Move.Root != root || *compiled.Snapshot().Rules[0].Move.Directory != `screenshots/${file.modified | date("month")}` {
			t.Fatal("mutable move snapshot escaped")
		}
	}
}

func TestOrganizationSchemaAndPathErrors(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	valid := fmt.Sprintf("version=1\n[[rules]]\nid='all'\n[rules.move]\nroot=%q\n", root)
	for name, source := range map[string]string{
		"source field":  valid + "[source]\npath='~/input'\n",
		"case root":     strings.Replace(valid, "root=", "Root=", 1),
		"relative root": strings.Replace(valid, fmt.Sprintf("root=%q", root), "root='relative'", 1),
		"folders":       strings.Replace(valid, "[[rules]]", "[selection]\nkind='both'\n[[rules]]", 1),
		"empty rename":  valid + "[rules.rename]\n", "both rename": valid + "[rules.rename]\nmode=''\nfilename='${file.name}'\n",
		"empty move": strings.Replace(valid, fmt.Sprintf("root=%q", root), "", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(source)); err == nil {
				t.Fatalf("invalid schema accepted: %s", source)
			}
		})
	}
	for _, root := range []string{"", "relative", "~", "~/", "~/../escape", "~//escape", `~/C:\escape`, "${file.name}", "~/safe/${index}", "~/safe/./child", "~/safe/../child", " ~/safe", "~/safe\n", `\\server\share`, "//server/share"} {
		if err := ValidateRoot(root); err == nil {
			t.Fatalf("unsafe root: %q", root)
		}
	}
	for _, directory := range []string{"", ".", "..", "../escape", "/absolute", "a/../b", "a//b", "a/", `a\b`, "C:/escape", "NUL", "CON.txt", "a:stream", "trailing.", "trailing ", "a\x00b", strings.Repeat("a/", MaxDirectoryDepth) + "a", strings.Repeat("a", 256)} {
		if _, err := Parse([]byte(valid + "directory=" + fmt.Sprintf("%q", directory) + "\n")); err == nil {
			t.Fatalf("unsafe directory: %q", directory)
		}
	}
	for _, extra := range []string{"source: null\n", "source: {path: '~/input'}\n", "source: {Path: '~/input'}\n"} {
		if _, err := ParseYAML([]byte(extra + "version: 1\nrules:\n - id: all\n   rename: {mode: snake}\n")); err == nil {
			t.Fatalf("invalid source accepted: %s", extra)
		}
	}
}

func TestDirectoryRenderRejectsEscapesAndBudget(t *testing.T) {
	program, err := compileDirectory("photos/${file.stem}")
	if err != nil {
		t.Fatal(err)
	}
	decision := Decision{RuleID: "all", directory: program}
	for _, name := range []string{"..", ".", "NUL", "a/b", `a\b`, "CON.txt", "C:drive", "trailing.", "bad\nname"} {
		if value, err := decision.RenderDirectory(render.Context{Name: name, Directory: true}); err == nil || value != "" {
			t.Fatalf("escape admitted: %q => %q %v", name, value, err)
		}
	}
	program, err = compileDirectory(strings.TrimSuffix(strings.Repeat("${file.name}/", 5), "/"))
	if err != nil {
		t.Fatal(err)
	}
	decision.directory = program
	if value, err := decision.RenderDirectory(render.Context{Name: strings.Repeat("a", 255)}); err == nil || value != "" {
		t.Fatalf("directory budget ignored: %d %v", len(value), err)
	}
}

func FuzzDirectoryPattern(f *testing.F) {
	for _, source := range []string{"photos/${file.modified | date(\"month\")}", "${file.stem | snake}/${index | pad(3)}", "../escape", "", "$${literal}/photos"} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		program, err := compileDirectory(source)
		if err != nil {
			return
		}
		stamp := time.Date(2026, 10, 6, 1, 2, 3, 0, time.UTC)
		size := int64(123)
		decision := Decision{RuleID: "all", directory: program}
		ctx := render.Context{Name: "Screenshot One.PNG", Index: 7, Modified: &stamp, Size: &size}
		value, err := decision.RenderDirectory(ctx)
		second, other := decision.RenderDirectory(ctx)
		if value != second || fmt.Sprint(err) != fmt.Sprint(other) {
			t.Fatal("nondeterministic directory")
		}
		if err != nil {
			if value != "" {
				t.Fatal("partial directory on failure")
			}
			return
		}
		local, err := filepath.Localize(value)
		if err != nil || !filepath.IsLocal(local) || len(value) > MaxDirectoryBytes {
			t.Fatalf("uncontained output: %q %v", value, err)
		}
		for _, component := range strings.Split(value, "/") {
			if err := ValidateDirectoryComponent(component); err != nil {
				t.Fatal(err)
			}
		}
	})
}
