package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestModeAndFilenameRulesShareOneConflictPlan(t *testing.T) {
	root, _, service, req := setup(t)
	a, b := filepath.Join(root, "A File.txt"), filepath.Join(root, "A-File.txt")
	put(t, a, "mode bytes")
	put(t, b, "filename bytes")
	req = withPreset(t, req, `version=1
[[rules]]
id='spaces'
[rules.match]
glob=['* *']
[rules.rename]
mode='snake'
[[rules]]
id='filename'
[rules.rename]
filename='${file.stem | snake}${file.ext}'
`)
	plan, err := service.Plan(context.Background(), req)
	if err != nil || len(plan.Result.Operations) != 1 || len(plan.Result.Collisions) != 1 || len(plan.Matches) != 2 || plan.Matches[0].Filename || !plan.Matches[1].Filename {
		t.Fatalf("separate action types bypassed global conflicts: %+v, %v", plan, err)
	}
	if _, err := service.Execute(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, filepath.Join(root, "a_file.txt"), "mode bytes")
	bytesAt(t, b, "filename bytes")
	if _, err := service.Undo(context.Background(), root, false); err != nil {
		t.Fatal(err)
	}
	bytesAt(t, a, "mode bytes")
	bytesAt(t, b, "filename bytes")
}
