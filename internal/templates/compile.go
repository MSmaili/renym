package templates

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/MSmaili/renym/internal/templates/render"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

// Compiled contains no mutable state or callable user code. All patterns and
// modes are validated once; matching uses the standard library's path.Match.
type Compiled struct {
	spec        Spec
	programs    []*render.Program
	directories []*directoryProgram
	hasMoves    bool
	sourcePath  string
}

// SourcePath is the resolved origin of a loaded template; Parse-only results
// have no origin. Application plans protect and journal this actual path.
func (c *Compiled) SourcePath() string { return c.sourcePath }

type Decision struct {
	RuleID    string
	Mode      string
	program   *render.Program
	moveRoot  string
	directory *directoryProgram
}

func (c *Compiled) HasMoves() bool  { return c.hasMoves }
func (d Decision) Moves() bool      { return d.moveRoot != "" }
func (d Decision) MoveRoot() string { return d.moveRoot }

func (d Decision) RendersFilename() bool { return d.program != nil }
func (d Decision) Dependencies() render.Dependencies {
	var result render.Dependencies
	include := func(program *render.Program) {
		if program == nil {
			return
		}
		dep := program.Dependencies()
		result.Modified = result.Modified || dep.Modified
		result.Size = result.Size || dep.Size
		result.Index = result.Index || dep.Index
	}
	include(d.program)
	if d.directory != nil {
		for _, program := range d.directory.parts {
			include(program)
		}
	}
	return result
}
func (d Decision) Render(ctx render.Context) (string, error) {
	if d.program == nil {
		return "", fmt.Errorf("rule %q does not render a filename", d.RuleID)
	}
	name, err := d.program.Render(ctx)
	if err != nil {
		return "", fmt.Errorf("rule %q rename.filename: %w", d.RuleID, err)
	}
	return name, nil
}

func compile(doc document) (*Compiled, error) {
	if doc.Version != Version {
		return nil, fmt.Errorf("version: required version is %d (got %d)", Version, doc.Version)
	}
	spec := Spec{Version: doc.Version, Selection: Selection{Kind: "files", Recursive: doc.Selection.Recursive, Ignore: doc.Selection.Ignore, NoDefaultIgnore: doc.Selection.NoDefaultIgnore}, Rules: doc.Rules}
	if doc.Name != nil {
		if strings.TrimSpace(*doc.Name) != *doc.Name || *doc.Name == "" || len(*doc.Name) > 128 || strings.IndexFunc(*doc.Name, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("name: must be a nonempty label of at most 128 bytes without surrounding whitespace or control characters")
		}
		spec.Name = *doc.Name
	}
	if doc.Selection.Kind != nil {
		spec.Selection.Kind = *doc.Selection.Kind
	}
	if !slices.Contains([]string{"files", "directories", "both"}, spec.Selection.Kind) {
		return nil, fmt.Errorf("selection.kind: expected files, directories, or both")
	}
	if err := validateGlobs("selection.ignore", spec.Selection.Ignore); err != nil {
		return nil, err
	}
	if len(spec.Rules) == 0 || len(spec.Rules) > MaxRules {
		return nil, fmt.Errorf("rules: expected 1 to %d rules", MaxRules)
	}
	seen := make(map[string]bool, len(spec.Rules))
	programs := make([]*render.Program, len(spec.Rules))
	directories := make([]*directoryProgram, len(spec.Rules))
	hasMoves := false
	for i, rule := range spec.Rules {
		field := fmt.Sprintf("rules[%d] (%q)", i+1, rule.ID)
		if !identifier.MatchString(rule.ID) {
			return nil, fmt.Errorf("%s.id: expected a 1-64 byte ASCII identifier (letters, digits, _, -, .)", field)
		}
		if seen[rule.ID] {
			return nil, fmt.Errorf("%s.id: duplicate rule ID", field)
		}
		seen[rule.ID] = true
		if err := validateGlobs(field+".match.glob", rule.Match.Glob); err != nil {
			return nil, err
		}
		if rule.Match.Glob != nil && len(rule.Match.Glob) == 0 {
			return nil, fmt.Errorf("%s.match.glob: explicit list must not be empty", field)
		}
		if rule.Match.Extensions != nil && (len(rule.Match.Extensions) == 0 || len(rule.Match.Extensions) > MaxPatterns) {
			return nil, fmt.Errorf("%s.match.extensions: expected 1 to %d values", field, MaxPatterns)
		}
		for _, ext := range rule.Match.Extensions {
			if len(ext) < 2 || len(ext) > 255 || ext[0] != '.' || strings.Count(ext, ".") != 1 || strings.ToLower(ext) != ext || strings.ContainsAny(ext, `/\*?[]`) || strings.IndexFunc(ext, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
				return nil, fmt.Errorf("%s.match.extensions: %q must be a lowercase dot-prefixed last extension, such as .png", field, ext)
			}
		}
		if rule.Move != nil {
			hasMoves = true
			if err := ValidateRoot(rule.Move.Root); err != nil {
				return nil, fmt.Errorf("%s.move.root: %w", field, err)
			}
			if rule.Move.Directory != nil {
				var err error
				directories[i], err = compileDirectory(*rule.Move.Directory)
				if err != nil {
					return nil, fmt.Errorf("%s.move.directory: %w", field, err)
				}
			}
		}
		if rule.Rename.Filename != nil {
			program, err := render.Compile(*rule.Rename.Filename)
			if err != nil {
				return nil, fmt.Errorf("%s.rename.filename: %w", field, err)
			}
			programs[i] = program
		} else if (rule.Rename.Mode != "" || rule.Move == nil) && !slices.Contains([]string{"upper", "lower", "pascal", "camel", "snake", "kebab", "title", "screaming", "sentence"}, rule.Rename.Mode) {
			return nil, fmt.Errorf("%s.rename.mode: unknown mode %q", field, rule.Rename.Mode)
		}
	}
	if hasMoves && spec.Selection.Kind != "files" {
		return nil, fmt.Errorf("move templates require selection.kind=files")
	}
	return &Compiled{spec: spec, programs: programs, directories: directories, hasMoves: hasMoves}, nil
}

func validateGlobs(field string, globs []string) error {
	if len(globs) > MaxPatterns {
		return fmt.Errorf("%s: at most %d patterns are allowed", field, MaxPatterns)
	}
	for _, glob := range globs {
		if glob == "" || len(glob) > MaxPatternBytes || strings.Contains(glob, "/") || strings.Contains(glob, "**") || strings.IndexByte(glob, 0) >= 0 {
			return fmt.Errorf("%s: %q must be a basename glob of 1-%d bytes without /, NUL, or **", field, glob, MaxPatternBytes)
		}
		if _, err := path.Match(glob, ""); err != nil {
			return fmt.Errorf("%s: invalid glob %q: %w", field, glob, err)
		}
	}
	return nil
}

func (c *Compiled) Selection() Selection {
	selection := c.spec.Selection
	selection.Ignore = slices.Clone(selection.Ignore)
	return selection
}

func (c *Compiled) Snapshot() Spec {
	spec := c.spec
	spec.Selection = c.Selection()
	spec.Rules = slices.Clone(c.spec.Rules)
	for i := range spec.Rules {
		if move := spec.Rules[i].Move; move != nil {
			copy := *move
			if move.Directory != nil {
				directory := *move.Directory
				copy.Directory = &directory
			}
			spec.Rules[i].Move = &copy
		}
		if filename := spec.Rules[i].Rename.Filename; filename != nil {
			copy := *filename
			spec.Rules[i].Rename.Filename = &copy
		}
		spec.Rules[i].Match.Glob = slices.Clone(spec.Rules[i].Match.Glob)
		spec.Rules[i].Match.Extensions = slices.Clone(spec.Rules[i].Match.Extensions)
	}
	return spec
}

// Match chooses exactly one action from the original basename/kind. Matching
// never cascades through generated names, even if the first action is a no-op.
func (c *Compiled) Match(basename string, directory bool) (Decision, bool) {
	for i, rule := range c.spec.Rules {
		if len(rule.Match.Glob) > 0 {
			matched := false
			for _, glob := range rule.Match.Glob {
				if ok, _ := path.Match(glob, basename); ok {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if len(rule.Match.Extensions) > 0 {
			ext := ""
			if !directory {
				ext = path.Ext(basename)
				if ext == basename {
					ext = ""
				} // .env is not an extension.
			}
			if !slices.Contains(rule.Match.Extensions, strings.ToLower(ext)) {
				continue
			}
		}
		decision := Decision{RuleID: rule.ID, Mode: rule.Rename.Mode, program: c.programs[i], directory: c.directories[i]}
		if rule.Move != nil {
			decision.moveRoot = rule.Move.Root
		}
		return decision, true
	}
	return Decision{}, false
}
