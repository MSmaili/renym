package templates

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

// Compiled contains no mutable state or callable user code. All patterns and
// modes are validated once; matching uses the standard library's path.Match.
type Compiled struct{ spec Spec }

type Decision struct {
	RuleID string
	Mode   string
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
		if !slices.Contains([]string{"upper", "lower", "pascal", "camel", "snake", "kebab", "title", "screaming", "sentence"}, rule.Rename.Mode) {
			return nil, fmt.Errorf("%s.rename.mode: unknown mode %q", field, rule.Rename.Mode)
		}
	}
	return &Compiled{spec: spec}, nil
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
		spec.Rules[i].Match.Glob = slices.Clone(spec.Rules[i].Match.Glob)
		spec.Rules[i].Match.Extensions = slices.Clone(spec.Rules[i].Match.Extensions)
	}
	return spec
}

// Match chooses exactly one action from the original basename/kind. Matching
// never cascades through generated names, even if the first action is a no-op.
func (c *Compiled) Match(basename string, directory bool) (Decision, bool) {
	for _, rule := range c.spec.Rules {
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
		return Decision{RuleID: rule.ID, Mode: rule.Rename.Mode}, true
	}
	return Decision{}, false
}
