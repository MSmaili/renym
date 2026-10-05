package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/MSmaili/renym/internal/cli"
	"github.com/MSmaili/renym/internal/log"
	"github.com/spf13/cobra"
)

func presetFlagCommand(t *testing.T) *cobra.Command {
	t.Helper()
	oldMode, oldTemplate, oldRecursive, oldDirectories, oldDirsOnly := mode, templatePath, recursive, directories, dirsOnly
	oldIgnore, oldDefault, oldVersion := ignore, noDefaultIgnore, showVersion
	t.Cleanup(func() {
		mode, templatePath, recursive, directories, dirsOnly = oldMode, oldTemplate, oldRecursive, oldDirectories, oldDirsOnly
		ignore, noDefaultIgnore, showVersion = oldIgnore, oldDefault, oldVersion
	})
	showVersion = false
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringVar(&mode, "mode", "", "")
	cmd.Flags().StringVar(&templatePath, "template", "", "")
	cmd.Flags().BoolVar(&recursive, "recursive", false, "")
	cmd.Flags().BoolVar(&directories, "directories", false, "")
	cmd.Flags().BoolVar(&dirsOnly, "dirs-only", false, "")
	cmd.Flags().StringSliceVar(&ignore, "ignore", nil, "")
	cmd.Flags().BoolVar(&noDefaultIgnore, "no-default-ignore", false, "")
	cmd.SetContext(context.Background())
	return cmd
}

func TestTemplateAndModeFlagsAreExclusiveEvenWithEmptyMode(t *testing.T) {
	for _, selectedMode := range []string{"snake", ""} {
		t.Run(selectedMode, func(t *testing.T) {
			cmd := presetFlagCommand(t)
			if err := cmd.ParseFlags([]string{"--template=example.toml", "--mode=" + selectedMode}); err != nil {
				t.Fatal(err)
			}
			if err := validateFlags(cmd, nil); !errors.Is(err, cli.ErrConflictingFlags) {
				t.Fatalf("conflicting selectors accepted: %v", err)
			}
		})
	}
}

func TestTemplateSelectorDefersInputDiscoveryToApplication(t *testing.T) {
	cmd := presetFlagCommand(t)
	if err := cmd.ParseFlags([]string{"--template=example.toml"}); err != nil {
		t.Fatal(err)
	}
	if err := validateFlags(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("template", ""); err != nil {
		t.Fatal(err)
	}
	if err := validateFlags(cmd, nil); err == nil {
		t.Fatal("empty selector accepted")
	}
}

func TestTemplateOverridesUseChangedFlagsIncludingFalseAndEmpty(t *testing.T) {
	cmd := presetFlagCommand(t)
	defaults := templateSelectionOverrides(cmd)
	if defaults.Kind != nil || defaults.Recursive != nil || defaults.Ignore != nil || defaults.NoDefaultIgnore != nil {
		t.Fatal("unselected CLI defaults overrode preset")
	}
	if err := cmd.ParseFlags([]string{"--directories=false", "--recursive=false", "--ignore=", "--no-default-ignore=false"}); err != nil {
		t.Fatal(err)
	}
	overrides := templateSelectionOverrides(cmd)
	if overrides.Kind == nil || *overrides.Kind != "files" || overrides.Recursive == nil || *overrides.Recursive || overrides.Ignore == nil || len(*overrides.Ignore) != 0 || overrides.NoDefaultIgnore == nil || *overrides.NoDefaultIgnore {
		t.Fatalf("explicit false/empty overrides lost: %+v", overrides)
	}
	if err := cmd.Flags().Set("directories", "true"); err != nil {
		t.Fatal(err)
	}
	if kind := *templateSelectionOverrides(cmd).Kind; kind != "both" {
		t.Fatalf("--directories selected %s", kind)
	}
	if err := cmd.Flags().Set("dirs-only", "true"); err != nil {
		t.Fatal(err)
	}
	if kind := *templateSelectionOverrides(cmd).Kind; kind != "directories" {
		t.Fatalf("--dirs-only selected %s", kind)
	}
}

func TestValidateTemplateDoesNotCreateHistory(t *testing.T) {
	root, config := t.TempDir(), t.TempDir()
	t.Setenv("HOME", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("APPDATA", config)
	filename := filepath.Join(root, "preset.toml")
	data := "version=1\n[[rules]]\nid='all'\n[rules.rename]\nmode='snake'\n"
	if err := os.WriteFile(filename, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stdout) })
	if err := validateTemplateCmd.RunE(cmd, []string{filename}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(config)
	if err != nil || len(entries) != 0 {
		t.Fatalf("validation created state: %v, %v", entries, err)
	}
	got, err := os.ReadFile(filename)
	if err != nil || string(got) != data {
		t.Fatal("validation modified preset")
	}
}
