package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApply(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		ops      []RenameOp

		test func(t *testing.T, root string, opsAbs []RenameOp, err error)
	}{
		{
			name:     "simple rename",
			existing: []string{"a.txt"},
			ops: []RenameOp{
				{OldPath: "a.txt", NewPath: "b.txt"},
			},
			test: func(t *testing.T, root string, ops []RenameOp, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				for _, op := range ops {
					if _, err := os.Stat(op.OldPath); !os.IsNotExist(err) {
						t.Errorf("old path still exists: %s", op.OldPath)
					}
					if _, err := os.Stat(op.NewPath); err != nil {
						t.Errorf("new path missing: %s", op.NewPath)
					}
				}
			},
		},
		{
			name:     "multiple renames",
			existing: []string{"x.txt", "y.txt"},
			ops: []RenameOp{
				{OldPath: "x.txt", NewPath: "x1.txt"},
				{OldPath: "y.txt", NewPath: "y1.txt"},
			},
			test: func(t *testing.T, root string, ops []RenameOp, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				for _, op := range ops {
					if _, err := os.Stat(op.NewPath); err != nil {
						t.Errorf("expected new file: %s", op.NewPath)
					}
				}
			},
		},
		{
			name:     "rename missing file should error",
			existing: []string{"exists.txt"},
			ops: []RenameOp{
				{OldPath: "missing.txt", NewPath: "new.txt"},
			},
			test: func(t *testing.T, root string, ops []RenameOp, err error) {
				if err == nil {
					t.Fatalf("expected error but got success")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()

			createFiles(t, root, tt.existing)

			opsAbs := make([]RenameOp, len(tt.ops))
			for i, op := range tt.ops {
				opsAbs[i] = RenameOp{
					OldPath: filepath.Join(root, op.OldPath),
					NewPath: filepath.Join(root, op.NewPath),
				}
			}

			err := Apply(opsAbs)

			tt.test(t, root, opsAbs, err)
		})
	}
}

// TODO: think where should this file belong?? we have same in two tests
// probably a common helpers
func createFiles(t *testing.T, root string, files []string) {
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
