package templates

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	toml "github.com/pelletier/go-toml/v2"
)

// Load resolves an explicit path or configured name, reads a bounded regular
// file, and compiles it. It never creates directories or runs template actions.
func Load(reference string) (*Compiled, error) {
	filename, err := Resolve(reference)
	if err != nil {
		return nil, fmt.Errorf("template %q: %w", reference, err)
	}
	format := strings.ToLower(filepath.Ext(filename))
	if !supportedExtension(format) {
		return nil, fmt.Errorf("template %q: supported formats are .toml, .yaml, and .yml", filename)
	}
	// Check ordinary special files before opening: opening a FIFO for reading
	// could otherwise block indefinitely. Hostile path swaps are out of scope.
	info, err := os.Stat(filename)
	if err != nil {
		return nil, fmt.Errorf("template %q: %w", filename, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("template %q: expected a regular file", filename)
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("template %q: %w", filename, err)
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, fmt.Errorf("template %q: %w", filename, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("template %q: expected a regular file", filename)
	}
	if info.Size() > MaxBytes {
		return nil, fmt.Errorf("template %q: exceeds %d-byte limit", filename, MaxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("template %q: %w", filename, err)
	}
	var compiled *Compiled
	if format == ".toml" {
		compiled, err = Parse(data)
	} else {
		compiled, err = ParseYAML(data)
	}
	if err != nil {
		return nil, fmt.Errorf("template %q: %w", filename, err)
	}
	compiled.sourcePath = filename
	return compiled, nil
}

// Parse strictly decodes, validates, and freezes a bounded TOML document.
func Parse(data []byte) (*Compiled, error) {
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("exceeds %d-byte limit", MaxBytes)
	}
	if !utf8.Valid(data) {
		return nil, errors.New("TOML must be valid UTF-8")
	}
	var doc document
	if err := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&doc); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			return nil, fmt.Errorf("invalid TOML: %s", strict.String())
		}
		var decode *toml.DecodeError
		if errors.As(err, &decode) {
			return nil, fmt.Errorf("invalid TOML: %s", decode.String())
		}
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	// Struct decoding deliberately accepts case-insensitive aliases. TOML keys
	// are case-sensitive, so inspect the preserved keys too; version/Version
	// must not overwrite one another in our schema. Both passes are byte-bounded.
	var keys map[string]any
	if err := toml.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	if err := validateKeys(keys); err != nil {
		return nil, err
	}
	return compile(doc)
}
