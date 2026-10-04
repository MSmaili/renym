package main

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/MSmaili/renym/internal/log"
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
