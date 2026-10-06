package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type organizationCLI struct {
	binary, base, input, output, home, policy string
}

func buildOrganizationCLI(t *testing.T) string {
	return buildCLIForAcceptance(t, true)
}

func buildCLIForAcceptance(t *testing.T, acceptance bool) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "renym")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	args := []string{"build", "-o", binary}
	if acceptance {
		args = append(args, "-tags=renym_organization_acceptance")
	}
	args = append(args, ".")
	output, err := exec.Command("go", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	return binary
}

func newOrganizationCLI(t *testing.T, binary, format string) organizationCLI {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := organizationCLI{binary: binary, base: base, input: filepath.Join(base, "input"), output: filepath.Join(base, "output"), home: filepath.Join(base, "home"), policy: filepath.Join(base, "policy."+format)}
	for _, dir := range []string{f.input, f.home} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func cliPut(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func (f organizationCLI) run(t *testing.T, wantSuccess bool, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, f.binary, args...)
	command.Dir = f.base
	command.Env = append(os.Environ(), "HOME="+f.home, "XDG_CONFIG_HOME="+f.home, "APPDATA="+f.home, "USERPROFILE="+f.home)
	output, err := command.CombinedOutput()
	if (err == nil) != wantSuccess {
		t.Fatalf("CLI %q: %v\n%s", args, err, output)
	}
	return string(output)
}

func cliTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			tree[rel] = "directory"
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			tree[rel] = "link:" + target
			return err
		}
		data, err := os.ReadFile(path)
		tree[rel] = "file:" + string(data)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return tree
}

func cliRequireText(t *testing.T, output string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(output, fragment) {
			t.Fatalf("missing %q:\n%s", fragment, output)
		}
	}
}

func cliRunID(t *testing.T, output string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "Run: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Run: "))
		}
	}
	t.Fatalf("run ID missing:\n%s", output)
	return ""
}

func TestOrganizationCLIAcceptance(t *testing.T) {
	binary := buildOrganizationCLI(t)
	for _, format := range []string{"toml", "yaml"} {
		t.Run(format, func(t *testing.T) {
			f := newOrganizationCLI(t, binary, format)
			cliPut(t, filepath.Join(f.input, "Screenshot One.PNG"), "image bytes")
			cliPut(t, filepath.Join(f.input, "Notes.txt"), "document bytes")
			cliPut(t, filepath.Join(f.input, "Other File.md"), "rename bytes")
			stamp := time.Date(2026, 10, 7, 1, 2, 3, 0, time.UTC)
			if err := os.Chtimes(filepath.Join(f.input, "Screenshot One.PNG"), stamp, stamp); err != nil {
				t.Fatal(err)
			}
			cliPut(t, f.policy, cliOrganizationPolicy(f))
			templateDir := filepath.Join(f.home, "renym", "templates")
			if err := os.MkdirAll(templateDir, 0700); err != nil {
				t.Fatal(err)
			}
			cliPut(t, filepath.Join(templateDir, "acceptance."+format), cliOrganizationPolicy(f))
			before := cliTree(t, f.base)
			preview := f.run(t, true, "--path", f.input, "--template", f.policy, "--dry-run")
			cliRequireText(t, preview, "Would create directory:", "Would move:", "Would rename:", "2026-10")
			if !reflect.DeepEqual(before, cliTree(t, f.base)) {
				t.Fatal("preview changed files, directories or history")
			}
			applied := f.run(t, true, "--path", f.input, "--template", "acceptance")
			cliRequireText(t, applied, "Files organized: 3", "Created directory:")
			id := cliRunID(t, applied)
			for name, content := range map[string]string{
				filepath.Join(f.output, "photos/2026-10/screenshot_one.png"): "image bytes",
				filepath.Join(f.base, "documents/Notes.txt"):                 "document bytes",
				filepath.Join(f.input, "other_file.md"):                      "rename bytes",
			} {
				data, err := os.ReadFile(name)
				if err != nil || string(data) != content {
					t.Fatalf("%s: %q %v", name, data, err)
				}
			}
			cliRequireText(t, f.run(t, true, "history"), id, "complete", f.input, "Recorded steps: 3")
			appliedTree := cliTree(t, f.base)
			cliRequireText(t, f.run(t, true, "undo", "--run", id, "--dry-run"), "UNDO PREVIEW", "Would move:", "Would check owned directory for empty cleanup:", "Would retain")
			if !reflect.DeepEqual(appliedTree, cliTree(t, f.base)) {
				t.Fatal("undo preview mutated the tree/journal")
			}
			undone := f.run(t, true, "undo", "--path", f.input)
			cliRequireText(t, undone, "UNDO COMPLETED", "Removed owned directory:", "Completed undo audit retained")
			for _, name := range []string{"Screenshot One.PNG", "Notes.txt", "Other File.md"} {
				key := filepath.Join("input", name)
				if cliTree(t, f.base)[key] != before[key] {
					t.Fatalf("original %s not restored", name)
				}
			}
			if _, err := os.Lstat(f.output); !os.IsNotExist(err) {
				t.Fatal("owned output retained")
			}
			cliRequireText(t, f.run(t, true, "history"), id, "organization_undone")
			f.run(t, false, "undo", "--run", id)
		})
	}
	t.Run("invalid-and-history-bypass", func(t *testing.T) {
		f := newOrganizationCLI(t, binary, "toml")
		cliPut(t, filepath.Join(f.input, "one.txt"), "original")
		cliPut(t, f.policy, fmt.Sprintf("version=1\n[[rules]]\nid='all'\n[rules.move]\nroot=%q\n", f.output))
		before := cliTree(t, f.base)
		cliRequireText(t, f.run(t, false, "--template", f.policy), "provide --path")
		cliRequireText(t, f.run(t, false, "--template", f.policy, "--path", f.input, "--skip-history"), "history", "--skip-history")
		f.run(t, false, "--template", f.policy, "--path", f.input, "--dirs-only")
		if !reflect.DeepEqual(before, cliTree(t, f.base)) {
			t.Fatal("invalid invocation mutated state")
		}
	})
	t.Run("conflict-preview-is-read-only", func(t *testing.T) {
		f := newOrganizationCLI(t, binary, "toml")
		cliPut(t, filepath.Join(f.input, "Same File.txt"), "one")
		cliPut(t, filepath.Join(f.input, "Same-File.txt"), "two")
		cliPut(t, f.policy, fmt.Sprintf("version=1\n[[rules]]\nid='all'\n[rules.rename]\nmode='snake'\n[rules.move]\nroot=%q\ndirectory='nested'\n", f.output))
		before := cliTree(t, f.base)
		preview := f.run(t, true, "--path", f.input, "--template", f.policy, "--dry-run")
		cliRequireText(t, preview, "COLLISIONS")
		if strings.Count(preview, "Would move:") != 1 || !reflect.DeepEqual(before, cliTree(t, f.base)) {
			t.Fatal("conflicting preview lost deterministic first-target selection or changed state")
		}
	})
	t.Run("all-occupied-targets-report-conflicts-without-mutation", func(t *testing.T) {
		f := newOrganizationCLI(t, binary, "toml")
		if err := os.Mkdir(f.output, 0700); err != nil {
			t.Fatal(err)
		}
		cliPut(t, filepath.Join(f.input, "one.txt"), "original")
		cliPut(t, filepath.Join(f.output, "one.txt"), "occupied")
		cliPut(t, f.policy, fmt.Sprintf("version=1\n[[rules]]\nid='all'\n[rules.move]\nroot=%q\n", f.output))
		before := cliTree(t, f.base)
		for _, args := range [][]string{
			{"--path", f.input, "--template", f.policy, "--dry-run"},
			{"--path", f.input, "--template", f.policy},
		} {
			cliRequireText(t, f.run(t, true, args...), "No organization changes planned", "COLLISIONS")
		}
		if !reflect.DeepEqual(before, cliTree(t, f.base)) {
			t.Fatal("occupied target modified files/history")
		}
	})
	t.Run("recovery-intent-visible-and-never-replayed", func(t *testing.T) {
		f := newOrganizationCLI(t, binary, "toml")
		cliPut(t, filepath.Join(f.input, "one.txt"), "original")
		cliPut(t, f.policy, fmt.Sprintf("version=1\n[[rules]]\nid='all'\n[rules.move]\nroot=%q\n", f.output))
		id := cliRunID(t, f.run(t, true, "--path", f.input, "--template", f.policy))
		journal := cliFindJournal(t, f.home, id)
		data, err := os.ReadFile(journal)
		if err != nil {
			t.Fatal(err)
		}
		var entry map[string]any
		if err := json.Unmarshal(data, &entry); err != nil {
			t.Fatal(err)
		}
		entry["state"] = "pending"
		entry["organization"].(map[string]any)["active"] = map[string]any{"action": "move", "operation_id": 1}
		data, err = json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(journal, data, 0600); err != nil {
			t.Fatal(err)
		}
		before := cliTree(t, f.base)
		cliRequireText(t, f.run(t, true, "history"), id, "pending", "Active intent: move", "Reconciliation required")
		cliRequireText(t, f.run(t, false, "undo", "--run", id), id, "Reconciliation required")
		cliRequireText(t, f.run(t, false, "undo", "--run", id, "--quiet"), id, "reconciliation required", "do not retry")
		cliRequireText(t, f.run(t, false, "undo", "--run", id, "--dry-run"), "reconciliation")
		cliPut(t, filepath.Join(f.input, "new.txt"), "new arrival")
		withArrival := cliTree(t, f.base)
		cliRequireText(t, f.run(t, false, "--path", f.input, "--template", f.policy), "reconciliation")
		if !reflect.DeepEqual(withArrival, cliTree(t, f.base)) {
			t.Fatal("apply replayed uncertain journal")
		}
		delete(withArrival, filepath.Join("input", "new.txt"))
		if !reflect.DeepEqual(before, withArrival) {
			t.Fatal("undo modified uncertain journal/files")
		}
	})
	t.Run("populated-cleanup-and-original-conflict", func(t *testing.T) {
		f := newOrganizationCLI(t, binary, "toml")
		cliPut(t, filepath.Join(f.input, "one.txt"), "original")
		cliPut(t, f.policy, fmt.Sprintf("version=1\n[[rules]]\nid='all'\n[rules.move]\nroot=%q\ndirectory='nested'\n", f.output))
		id := cliRunID(t, f.run(t, true, "--path", f.input, "--template", f.policy))
		cliPut(t, filepath.Join(f.input, "one.txt"), "new arrival")
		cliPut(t, filepath.Join(f.output, "nested/user.txt"), "user bytes")
		cliRequireText(t, f.run(t, false, "undo", "--run", id), "Run: "+id, "undo stopped")
		data, _ := os.ReadFile(filepath.Join(f.input, "one.txt"))
		if string(data) != "new arrival" {
			t.Fatal("undo overwrote arrival")
		}
		if err := os.Rename(filepath.Join(f.input, "one.txt"), filepath.Join(f.input, "arrival.txt")); err != nil {
			t.Fatal(err)
		}
		cliRequireText(t, f.run(t, true, "undo", "--run", id), "Retained directory:")
		data, _ = os.ReadFile(filepath.Join(f.output, "nested/user.txt"))
		if string(data) != "user bytes" {
			t.Fatal("cleanup removed populated directory")
		}
	})
}

func cliFindJournal(t *testing.T, root, id string) string {
	t.Helper()
	var found string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == id {
			found = path
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if found == "" {
		t.Fatalf("journal %s missing", id)
	}
	return found
}

func TestNormalCLIOrganizationGate(t *testing.T) {
	f := newOrganizationCLI(t, buildCLIForAcceptance(t, false), "toml")
	cliPut(t, filepath.Join(f.input, "one.txt"), "original")
	cliPut(t, f.policy, fmt.Sprintf("version=1\n[[rules]]\nid='all'\n[rules.move]\nroot=%q\n", f.output))
	before := cliTree(t, f.base)
	cliRequireText(t, f.run(t, false, "--path", f.input, "--template", f.policy), "gated", "--dry-run")
	if !reflect.DeepEqual(before, cliTree(t, f.base)) {
		t.Fatal("normal CLI gate mutated tree/history")
	}
	f.run(t, false, "undo", f.input)
	f.run(t, false, "undo", "--path", f.input, "--run", "invalid")
	f.run(t, false, "undo", "--run", "")
}

func cliOrganizationPolicy(f organizationCLI) string {
	documents := filepath.Join(f.base, "documents")
	if strings.HasSuffix(f.policy, ".yaml") {
		return "version: 1\nrules:\n - id: image\n   match: {extensions: ['.png']}\n   rename: {filename: '${file.stem | snake}${file.ext | lower}'}\n   move: {root: " + strconv.Quote(f.output) + ", directory: 'photos/${file.modified | date(\"month\")}'}\n - id: text\n   match: {extensions: ['.txt']}\n   move: {root: " + strconv.Quote(documents) + "}\n - id: remaining\n   rename: {mode: snake}\n"
	}
	return fmt.Sprintf(`version=1
[[rules]]
id='image'
[rules.match]
extensions=['.png']
[rules.rename]
filename='${file.stem | snake}${file.ext | lower}'
[rules.move]
root=%q
directory='photos/${file.modified | date("month")}'
[[rules]]
id='text'
[rules.match]
extensions=['.txt']
[rules.move]
root=%q
[[rules]]
id='remaining'
[rules.rename]
mode='snake'
`, f.output, documents)
}
