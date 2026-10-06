package templates

import (
	"fmt"
	"maps"
	"slices"
)

func exactKeys(table map[string]any, field string, allowed ...string) error {
	for _, key := range slices.Sorted(maps.Keys(table)) {
		if !slices.Contains(allowed, key) {
			return fmt.Errorf("%s: unknown field %q (schema keys are case-sensitive)", field, key)
		}
	}
	return nil
}

// Type and duplicate-definition validation belongs to the typed decoder. This
// checks key spelling only, including case aliases the decoder would accept.
func validateKeys(root map[string]any) error {
	if err := exactKeys(root, "template", "version", "name", "selection", "rules"); err != nil {
		return err
	}
	selection, _ := root["selection"].(map[string]any)
	if err := exactKeys(selection, "selection", "kind", "recursive", "ignore", "no_default_ignore"); err != nil {
		return err
	}
	rules, _ := root["rules"].([]any)
	for i, value := range rules {
		rule, _ := value.(map[string]any)
		field := fmt.Sprintf("rules[%d]", i+1)
		if err := exactKeys(rule, field, "id", "match", "rename", "move"); err != nil {
			return err
		}
		match, _ := rule["match"].(map[string]any)
		if err := exactKeys(match, field+".match", "glob", "extensions"); err != nil {
			return err
		}
		rename, _ := rule["rename"].(map[string]any)
		move, _ := rule["move"].(map[string]any)
		if err := exactKeys(move, field+".move", "root", "directory"); err != nil {
			return err
		}
		if err := exactKeys(rename, field+".rename", "mode", "filename"); err != nil {
			return err
		}
		_, mode := rename["mode"]
		_, filename := rename["filename"]
		_, hasRename := rule["rename"]
		_, hasMove := rule["move"]
		if !hasRename && !hasMove {
			return fmt.Errorf("%s: supply at least one of rename or move", field)
		}
		if hasRename && mode == filename {
			return fmt.Errorf("%s.rename: supply exactly one of mode or filename", field)
		}
	}
	return nil
}
