package templates

import (
	"strings"
	"testing"

	"github.com/MSmaili/renym/internal/templates/render"
)

func TestFilenameSchemaAndImmutableSnapshot(t *testing.T) {
	compiled, err := Parse([]byte("version=1\n[[rules]]\nid='all'\n[rules.rename]\nfilename='${file.stem | snake}${file.ext}'\n"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := compiled.Snapshot()
	if snapshot.Rules[0].Rename.Filename == nil || *snapshot.Rules[0].Rename.Filename != "${file.stem | snake}${file.ext}" {
		t.Fatalf("filename not snapshotted: %+v", snapshot)
	}
	*snapshot.Rules[0].Rename.Filename = "changed"
	decision, matched := compiled.Match("My File.txt", false)
	if !matched || !decision.RendersFilename() || decision.Mode != "" {
		t.Fatalf("incorrect action: %+v", decision)
	}
	name, err := decision.Render(render.Context{Name: "My File.txt"})
	if err != nil || name != "my_file.txt" {
		t.Fatalf("snapshot mutated renderer: %q, %v", name, err)
	}
	if *compiled.Snapshot().Rules[0].Rename.Filename != "${file.stem | snake}${file.ext}" {
		t.Fatal("snapshot mutated compiled spec")
	}
}

func TestFilenameSchemaErrors(t *testing.T) {
	for _, rename := range []string{
		"", "filename=''", "filename=3", "filename='${index | snake}'",
		"mode='snake'\nfilename='${file.name}'", "mode=''\nfilename='${file.name}'",
		"mode='snake'\nfilename=''", "Filename='${file.name}'", "filename='${file.created}'",
	} {
		source := "version=1\n[[rules]]\nid='all'\n[rules.rename]\n" + rename + "\n"
		if compiled, err := Parse([]byte(source)); err == nil || compiled != nil {
			t.Fatalf("invalid rename action accepted: %q", rename)
		}
	}
	_, err := Parse([]byte("version=1\n[[rules]]\nid='all'\n[rules.rename]\nfilename='prefix_${unknown}'\n"))
	if err == nil || !strings.Contains(err.Error(), `("all").rename.filename`) || !strings.Contains(err.Error(), "filename byte 10") {
		t.Fatalf("missing decoded filename diagnostic context: %v", err)
	}
}
