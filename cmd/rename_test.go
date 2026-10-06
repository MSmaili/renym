package main

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MSmaili/renym/internal/app"
	"github.com/MSmaili/renym/internal/engine"
	"github.com/MSmaili/renym/internal/log"
	"github.com/MSmaili/renym/internal/templates"
)

func TestDryRunDoesNotCreateHistory(t *testing.T) {
	root, configHome := t.TempDir(), t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("APPDATA", configHome)
	file := filepath.Join(root, "Hello World.txt")
	if err := os.WriteFile(file, []byte("keep these bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	oldPath, oldMode, oldGlobal := path, mode, globalCfg
	oldContext := rootCmd.Context()
	rootCmd.SetContext(context.Background())
	path, mode, globalCfg.DryRun = root, "snake", true
	log.SetOutput(io.Discard)
	t.Cleanup(func() {
		path, mode, globalCfg = oldPath, oldMode, oldGlobal
		rootCmd.SetContext(oldContext)
		log.SetOutput(os.Stdout)
	})
	if err := runRename(rootCmd, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "keep these bytes" {
		t.Fatalf("preview changed source: %q, %v", data, err)
	}
	if err := filepath.WalkDir(configHome, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != configHome {
			t.Errorf("preview created configuration/history: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRuleActionLabel(t *testing.T) {
	for _, test := range []struct {
		match app.RuleMatch
		want  string
	}{
		{app.RuleMatch{Mode: "snake"}, "snake"},
		{app.RuleMatch{Filename: true}, "filename"},
		{app.RuleMatch{Move: true}, "move"},
		{app.RuleMatch{Mode: "snake", Move: true}, "snake+move"},
		{app.RuleMatch{Filename: true, Move: true}, "filename+move"},
	} {
		if got := ruleActionLabel(test.match); got != test.want {
			t.Errorf("ruleActionLabel(%+v) = %q, want %q", test.match, got, test.want)
		}
	}
}

func TestRenameRequestInputPresence(t *testing.T) {
	for _, test := range []struct {
		flags []string
		want  string
	}{
		{[]string{"--template=policy.yaml"}, ""},
		{[]string{"--template=policy.yaml", "--path=."}, "."},
		{[]string{"--template=policy.yaml", "--path=inbox"}, "inbox"},
		{[]string{"--mode=snake"}, "."},
	} {
		cmd := presetFlagCommand(t)
		if err := cmd.ParseFlags(test.flags); err != nil {
			t.Fatal(err)
		}
		if got := renameRequest(cmd).Path; got != test.want {
			t.Errorf("flags %v: input %q, want %q", test.flags, got, test.want)
		}
	}
}

func TestOrganizationCommandPreviewRequiresInputAndDoesNotMutate(t *testing.T) {
	// Preview paths are canonicalized, including Windows short temp-directory
	// aliases and macOS /var links. Build expected targets from the same origin.
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	input, output, policy := filepath.Join(base, "inbox"), filepath.Join(base, "sorted"), filepath.Join(base, "policy.yaml")
	if err := os.Mkdir(input, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(input, "Some File.txt")
	if err := os.WriteFile(file, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	data := "version: 1\nrules:\n - id: all\n   move: {root: " + strconv.Quote(output) + "}\n"
	if err := os.WriteFile(policy, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := presetFlagCommand(t)
	oldGlobal, oldSkip, oldLevel := globalCfg, skipHistory, log.GetLevel()
	globalCfg.DryRun, skipHistory = true, true
	var messages bytes.Buffer
	log.SetOutput(&messages)
	log.SetLevel(log.LevelNormal)
	t.Cleanup(func() {
		globalCfg, skipHistory = oldGlobal, oldSkip
		log.SetLevel(oldLevel)
		log.SetOutput(os.Stdout)
	})
	if err := cmd.ParseFlags([]string{"--template=" + policy}); err != nil {
		t.Fatal(err)
	}
	if err := runRename(cmd, nil); err == nil || !strings.Contains(err.Error(), "provide --path") {
		t.Fatalf("omitted organization input accepted: %v", err)
	}
	if err := cmd.Flags().Set("path", input); err != nil {
		t.Fatal(err)
	}
	if err := runRename(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(messages.String(), "Would move:") || !strings.Contains(messages.String(), filepath.Join(output, "Some File.txt")) {
		t.Fatalf("missing preview: %s", messages.String())
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("preview created destination: %v", err)
	}
	contents, err := os.ReadFile(file)
	if err != nil || string(contents) != "unchanged" {
		t.Fatalf("preview mutated input: %q %v", contents, err)
	}
}

func TestRenamePlanOutput(t *testing.T) {
	var output bytes.Buffer
	oldLevel := log.GetLevel()
	log.SetLevel(log.LevelDebug)
	log.SetOutput(&output)
	t.Cleanup(func() {
		log.SetLevel(oldLevel)
		log.SetOutput(os.Stdout)
	})
	source := filepath.Join("input", "Some File.txt")
	destination := filepath.Join("output", "some_file.txt")
	plan := app.Plan{
		TemplatePath:        "policy.yaml",
		TemplateName:        "Organize",
		Selection:           templates.Selection{Kind: "files"},
		SourcePath:          "input",
		PreviewOnly:         true,
		Organization:        true,
		Overrides:           []string{"recursive"},
		Matches:             []app.RuleMatch{{Path: source, RuleID: "all", Filename: true, Move: true, Index: 1}},
		DirectoriesToCreate: []string{"output"},
		Result: engine.PlanResult{Operations: []engine.RenameOp{
			{OldPath: source, NewPath: destination},
			{OldPath: source, NewPath: filepath.Join("input", "some_file.txt")},
		}},
	}
	printTemplatePlan(plan)
	printRenamePreview(plan)
	want := "Template: policy.yaml (Organize)\n" +
		"Selection: kind=files recursive=false ignore=[] no_default_ignore=false\n" +
		"Source: input\n" +
		"Organization: regular-file moves; history required for apply.\n" +
		"CLI overrides: recursive\n" +
		"Rule all (filename+move, index=1): " + source + "\n" +
		"Would create directory: output\n" +
		"Would move: " + source + " -> " + destination + "\n" +
		"Would rename: " + source + " -> " + filepath.Join("input", "some_file.txt") + "\n"
	if output.String() != want {
		t.Fatalf("output changed:\ngot: %q\nwant: %q", output.String(), want)
	}
	output.Reset()
	printTemplatePlan(app.Plan{})
	if output.Len() != 0 {
		t.Fatal("non-template plan printed template metadata")
	}
	for _, test := range []struct {
		preview bool
		want    string
	}{
		{false, "\n✓ No files to rename\n"},
		{true, "\nNo organization changes planned\n"},
	} {
		output.Reset()
		printEmptyRenamePlan(app.Plan{PreviewOnly: test.preview, Organization: test.preview, Result: engine.PlanResult{
			Skipped: []engine.SkippedFile{{Path: source, Reason: "occupied target"}},
		}})
		if got := output.String(); got != test.want+"Skipped: "+source+" (occupied target)\n" {
			t.Fatalf("empty plan output changed: %q", got)
		}
	}
}
