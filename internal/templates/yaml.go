package templates

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

const (
	MaxYAMLNodes = 16384
	MaxYAMLDepth = 16
)

// ParseYAML enforces a small typed YAML subset before invoking the same compiler
// as TOML. Node validation prevents coercions/aliases before typed decoding.
func ParseYAML(data []byte) (*Compiled, error) {
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("exceeds %d-byte limit", MaxBytes)
	}
	if !utf8.Valid(data) {
		return nil, errors.New("YAML must be valid UTF-8")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var root yaml.Node
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("invalid trailing YAML: %w", err)
		}
		return nil, errors.New("YAML must contain exactly one document")
	}
	if len(root.Content) != 1 {
		return nil, errors.New("YAML must contain one template mapping")
	}
	budget := MaxYAMLNodes
	if err := checkYAML(root.Content[0], "template", "template", 1, &budget); err != nil {
		return nil, err
	}
	var doc document
	if err := root.Content[0].Decode(&doc); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return compile(doc)
}

func yamlError(node *yaml.Node, field, message string) error {
	return fmt.Errorf("YAML line %d, column %d: %s: %s", node.Line, node.Column, field, message)
}

func checkYAML(node *yaml.Node, shape, field string, depth int, budget *int) error {
	(*budget)--
	if *budget < 0 || depth > MaxYAMLDepth {
		return yamlError(node, field, "YAML node/depth limit exceeded")
	}
	if node.Anchor != "" || node.Kind == yaml.AliasNode || node.Tag == "!!merge" {
		return yamlError(node, field, "anchors, aliases, and merge keys are unsupported")
	}
	switch shape {
	case "string", "boolean", "integer":
		tag := "!!str"
		if shape == "boolean" {
			tag = "!!bool"
		} else if shape == "integer" {
			tag = "!!int"
		}
		if node.Kind != yaml.ScalarNode || node.Tag != tag {
			return yamlError(node, field, "expected a "+shape+" (no implicit coercion or null)")
		}
		if shape == "boolean" && node.Value != "true" && node.Value != "false" {
			return yamlError(node, field, "booleans must be lowercase true or false")
		}
		if shape == "integer" {
			text := node.Value
			if text == "" || len(text) > 1 && text[0] == '0' {
				return yamlError(node, field, "expected a canonical nonnegative decimal integer")
			}
			for _, c := range text {
				if c < '0' || c > '9' {
					return yamlError(node, field, "expected a canonical nonnegative decimal integer")
				}
			}
			if _, err := strconv.ParseInt(text, 10, 64); err != nil {
				return yamlError(node, field, "integer is out of range")
			}
		}
	case "strings", "rules":
		if node.Kind != yaml.SequenceNode || node.Tag != "!!seq" {
			return yamlError(node, field, "expected a sequence")
		}
		element := "string"
		if shape == "rules" {
			element = "rule"
		}
		for i, child := range node.Content {
			if err := checkYAML(child, element, fmt.Sprintf("%s[%d]", field, i+1), depth+1, budget); err != nil {
				return err
			}
		}
	default:
		if node.Kind != yaml.MappingNode || node.Tag != "!!map" {
			return yamlError(node, field, "expected a mapping")
		}
		seen := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if err := checkYAML(key, "string", field+" key", depth+1, budget); err != nil {
				return err
			}
			if seen[key.Value] {
				return yamlError(key, field, "duplicate key "+strconv.Quote(key.Value))
			}
			seen[key.Value] = true
			childShape := yamlFieldShape(shape, key.Value)
			if childShape == "" {
				return yamlError(key, field, "unknown field "+strconv.Quote(key.Value)+" (schema keys are case-sensitive)")
			}
			if err := checkYAML(value, childShape, field+"."+key.Value, depth+1, budget); err != nil {
				return err
			}
		}
		if shape == "rename" && seen["mode"] == seen["filename"] {
			return yamlError(node, field, "supply exactly one of mode or filename")
		}
	}
	return nil
}

func yamlFieldShape(parent, field string) string {
	switch parent {
	case "template":
		switch field {
		case "version":
			return "integer"
		case "name":
			return "string"
		case "selection":
			return "selection"
		case "rules":
			return "rules"
		}
	case "selection":
		switch field {
		case "kind":
			return "string"
		case "recursive", "no_default_ignore":
			return "boolean"
		case "ignore":
			return "strings"
		}
	case "rule":
		switch field {
		case "id":
			return "string"
		case "match":
			return "match"
		case "rename":
			return "rename"
		}
	case "match":
		switch field {
		case "glob", "extensions":
			return "strings"
		}
	case "rename":
		switch field {
		case "mode", "filename":
			return "string"
		}
	}
	return ""
}
