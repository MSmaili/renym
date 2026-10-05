# TOML rename presets

Reuse existing rename modes through an explicitly selected TOML file:

```sh
renym template validate ./examples/templates/screenshots.toml
renym --template ./examples/templates/screenshots.toml -p ./inbox --dry-run
renym --template ./examples/templates/screenshots.toml -p ./inbox
# Undo from the input directory:
cd ./inbox
renym undo --dry-run
renym undo
```

This is the initial mode-preset slice. YAML, named lookup, `template list`, filename expressions, moving, watching, and AI actions are **not supported yet**. Unsupported fields are errors, not ignored behavior. `--template` and `--mode` are exclusive, even when one is explicitly empty. No preset is automatically discovered or executed from a project directory.

## Schema v1

```toml
version = 1
name = "Screenshot names" # optional display label, not a lookup name

[selection]
kind = "files"           # files (default), directories, or both
recursive = false        # default: false
ignore = ["*.tmp"]       # additional basename ignores; default: []
no_default_ignore = false # retain the existing .git/.svn/.hg ignores

[[rules]]
id = "screenshots"
[rules.match]
glob = ["Screenshot*", "Screen Shot*"]
extensions = [".png"]
[rules.rename]
mode = "snake"
```

Keys are case-sensitive. Version 1 and at least one rule are required. Unknown fields, duplicate definitions/IDs, wrong value types, invalid patterns, and unknown modes fail before discovering input files. Parser errors include source locations when available; semantic errors identify the rule/field. Display labels, when present, must be nonempty and have no surrounding whitespace or control characters.

Each rule requires a unique 1–64 byte ID starting with an ASCII letter/digit and otherwise containing letters, digits, `_`, `-`, or `.`. `rename.mode` supports `upper`, `lower`, `pascal`, `camel`, `snake`, `kebab`, `title`, `screaming`, and `sentence`, preserving the existing mode behavior.

## Matching and naming

- Rules run in document order against **original** names and kinds. The **first match wins**, even if that action results in no change or a collision; there is no fallback/chaining after a match.
- Different match fields are ANDed. Values within a list are ORed. Omitting `match` or both its fields matches every selected item. Explicit empty match lists are invalid.
- `glob` and preset `ignore` use Go's [`path.Match`](https://pkg.go.dev/path#Match) basename semantics on every OS: case-sensitive `*`, `?`, character classes, and backslash escapes. `/` and recursive `**` patterns are not accepted. TOML literal strings are convenient for patterns containing backslashes.
- `extensions` contains lowercase, dot-prefixed **last** extensions such as `.png` or `.gz`. Matching is case-insensitive, but naming preserves the original extension's case. `archive.tar.gz` matches `.gz`, not `.tar.gz`. `.env` has no extension for matching; `.local.env` matches `.env`. Directories never match extension filters, even when their names contain dots.
- Mode actions retain existing stem/extension normalization; they are not filename-expression helpers. A dotted directory is normalized as a complete directory name. Existing dotfile mode behavior is unchanged.
- Unmatched entries are reported as skipped. Conflicts are resolved conservatively by the shared planner; existing destinations are never overwritten. Inputs are evaluated in lexical relative-path order; physical nested-directory renames remain deepest-first, and undo reverses completed physical steps.

The traversal root is never renamed. Passing a directory means selecting its contents (and descendants with recursion), not the directory itself. A single regular-file path is eligible only when files are selected and the preset's ignores and rules allow it. Symlink/special-file sources are unsupported. The active preset file and its ancestors are protected from being renamed by their own run.

## Explicit CLI overrides

Unspecified CLI defaults do **not** replace the preset's selection. Only explicitly supplied flags override the corresponding fields:

| Flag                             | Effect                                                                             |
| -------------------------------- | ---------------------------------------------------------------------------------- |
| `--recursive=true/false`         | Overrides recursion, including explicit `false`                                    |
| `--directories`                  | Selects files and directories                                                      |
| `--directories=false`            | Selects files only                                                                 |
| `--dirs-only`                    | Selects directories only; takes precedence over `--directories` when true          |
| `--dirs-only=false`              | Restores ordinary files-only selection unless `--directories` is also true         |
| `--ignore=PATTERN`               | Replaces the preset's additional ignore list; repeat or use comma-separated values |
| `--ignore=""`                    | Clears the preset's additional ignores, not the default ignores                    |
| `--no-default-ignore=true/false` | Overrides the default-ignore switch                                                |

The preview prints the effective selection and override names. `--verbose` also shows the chosen rule and mode for each matched item. Dry runs and `template validate` never write undo history. Apply stores a normalized preset snapshot alongside its effective request, so subsequent preset edits cannot change an already-created plan or its undo steps.

## Limits and safety

An explicit `.toml` file must be a regular file and contain at most 64 KiB of valid UTF-8. A preset may contain 1–128 rules, at most 32 glob/extension values per rule field and 32 additional ignores, and glob patterns of at most 1,024 bytes. Labels are limited to 128 bytes; extension values to 255 bytes. These are mode-preset limits, not a filename-renderer grammar contract.

The ordinary [rename/undo safety rules](../readme.md#rename-and-undo-safety) apply. In particular, preview is not a transaction or a promise that the filesystem will remain unchanged until apply; stale sources or newly occupied targets stop execution. History remains in its existing platform-native location. This slice does not create or search `~/.config/renym/templates` yet.
