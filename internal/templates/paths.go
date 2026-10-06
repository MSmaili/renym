package templates

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MSmaili/renym/internal/templates/render"
)

const MaxDirectoryBytes = 1024
const MaxDirectoryDepth = 16

// ValidateRoot admits literal native absolute paths or ~/... only. It does not
// read the environment, expand roots, or access the filesystem.
func ValidateRoot(root string) error {
	if root == "" || len(root) > 4096 || !utf8.ValidString(root) || strings.TrimSpace(root) != root || strings.IndexFunc(root, unicode.IsControl) >= 0 || strings.Contains(root, "${") {
		return fmt.Errorf("expected a bounded literal absolute or ~/... path without interpolation/control characters")
	}
	path := root
	if strings.HasPrefix(root, "//") || strings.HasPrefix(root, `\\`) {
		return fmt.Errorf("UNC/device roots are unsupported")
	}
	if strings.HasPrefix(root, "~/") {
		path = strings.TrimPrefix(root, "~/")
		if path == "" || strings.ContainsAny(path, `\:`) || strings.HasPrefix(path, "/") {
			return fmt.Errorf("~/ must include a slash-separated relative path beneath home")
		}
	} else if !filepath.IsAbs(root) {
		return fmt.Errorf("expected an absolute or ~/... path")
	}
	// Roots are deliberately not repaired/cleaned; a dot segment could disguise
	// a different authority. Both separators are checked for Windows portability.
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == "." || part == ".." {
			return fmt.Errorf("root dot segments are unsupported")
		}
	}
	return nil
}

func ValidateDirectoryComponent(component string) error {
	if component == "" || len(component) > render.MaxNameBytes || !utf8.ValidString(component) || strings.ContainsAny(component, `/\<>:"|?*`) || strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") || strings.IndexFunc(component, unicode.IsControl) >= 0 {
		return fmt.Errorf("invalid portable directory component %q", component)
	}
	base := strings.ToUpper(strings.SplitN(component, ".", 2)[0])
	for _, prefix := range []string{"COM", "LPT"} {
		for _, digit := range []string{"¹", "²", "³"} {
			if base == prefix+digit {
				return fmt.Errorf("reserved directory component %q", component)
			}
		}
	}
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return fmt.Errorf("reserved directory component %q", component)
	}
	return nil
}

type directoryProgram struct{ parts []*render.Program }

func compileDirectory(source string) (*directoryProgram, error) {
	if source == "" || len(source) > render.MaxSourceBytes {
		return nil, fmt.Errorf("expected 1-4096 bytes; omit directory to use the root directly")
	}
	components := strings.Split(source, "/")
	if len(components) > MaxDirectoryDepth {
		return nil, fmt.Errorf("at most %d directory components are allowed", MaxDirectoryDepth)
	}
	programs := make([]*render.Program, len(components))
	for i, component := range components {
		program, err := render.Compile(component)
		if err != nil {
			return nil, fmt.Errorf("component %d: %w", i+1, err)
		}
		if !strings.Contains(component, "${") {
			value, err := program.Render(render.Context{})
			if err != nil {
				return nil, fmt.Errorf("component %d: %w", i+1, err)
			}
			if err := ValidateDirectoryComponent(value); err != nil {
				return nil, fmt.Errorf("component %d: %w", i+1, err)
			}
		}
		programs[i] = program
	}
	return &directoryProgram{parts: programs}, nil
}

// RenderDirectory evaluates independently bounded components. A field cannot
// introduce separators or dot segments; separators are literal schema structure.
func (d Decision) RenderDirectory(ctx render.Context) (string, error) {
	if d.directory == nil {
		return "", nil
	}
	parts := make([]string, len(d.directory.parts))
	length := 0
	for i, program := range d.directory.parts {
		component, err := program.Render(ctx)
		if err == nil {
			err = ValidateDirectoryComponent(component)
		}
		if err != nil {
			return "", fmt.Errorf("rule %q move.directory component %d: %w", d.RuleID, i+1, err)
		}
		length += len(component)
		if i > 0 {
			length++
		}
		if length > MaxDirectoryBytes {
			return "", fmt.Errorf("rule %q move.directory exceeds %d bytes", d.RuleID, MaxDirectoryBytes)
		}
		parts[i] = component
	}
	return strings.Join(parts, "/"), nil
}
