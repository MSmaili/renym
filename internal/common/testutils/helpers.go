package testutils

import (
	"os"
	"path/filepath"
	"testing"
)

func CreateFiles(t *testing.T, root string, files []string) {
	t.Helper()
	for _, f := range files {
		path := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}
}
