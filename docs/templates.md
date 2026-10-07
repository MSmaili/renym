# TOML/YAML rename templates

Reuse existing rename modes through a selected TOML/YAML file or configured name:

```sh
renym template validate ./examples/templates/screenshots.toml
renym --template ./examples/templates/screenshots.toml -p ./inbox --dry-run
renym --template ./examples/templates/screenshots.toml -p ./inbox
# Undo from the input directory:
cd ./inbox
renym undo --dry-run
renym undo
```

Both formats support existing mode presets and [bounded filename patterns](filename-patterns.md), using the same schema, compiler, planner, and executor. [Manual organization](organization.md) adds per-rule move destinations with an explicit input folder supplied at invocation, same-filesystem apply and verified undo with mandatory history. Watching and AI actions are not supported yet. Templates do not declare `source`; unsupported fields are errors, not ignored behavior. `--template` and `--mode` are exclusive, even when one is explicitly empty. No preset is automatically discovered or executed from a project directory.

## Names and storage

Store editable files in:

- **macOS/Linux:** `$XDG_CONFIG_HOME/renym/templates` when the override is absolute; otherwise `~/.config/renym/templates`. Unset, empty, and relative XDG values use the home fallback. macOS does not use Application Support for templates.
- **Windows:** `os.UserConfigDir()/renym/templates`, normally `%APPDATA%\renym\templates`. XDG does not override this.

```sh
renym template list
renym template validate screenshots
renym --template screenshots -p ./inbox --dry-run
```

A bare name is the filename without `.toml`, `.yaml`, or `.yml`, not the optional display label. Names match exactly (case-sensitive), start with an ASCII letter/digit, and contain at most 64 bytes of letters, digits, `_`, `-`, or `.`. A name cannot end in a supported format suffix: those references always mean explicit paths. Suffix matching itself is case-insensitive. Any reference containing `/` or `\` also means an explicit path, relative to the working directory unless absolute. Use `./` to select an explicit path with an unsupported suffix and receive a format error. Renym does not expand `~` or environment variables inside references; use shell expansion for paths.

Named lookup searches only this directory, never the working directory or parent projects. If both `screenshots.toml` and `screenshots.yaml` (or `.yml`) exist, the name is ambiguous and fails; select an explicit path instead. Listing shows each eligible name and origin path, including ambiguity, without parsing contents. Subdirectories, unsupported suffixes, and ineligible names are omitted. Missing storage lists as empty; validation/loading still requires a regular file. Inspection does not create directories, install presets, fetch remote files, or migrate history. Create the directory and save your templates yourself.

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

Equivalent YAML ([executable example](../examples/templates/screenshots.yaml)):

```yaml
version: 1
name: Screenshot names
selection:
  kind: files
  recursive: false
rules:
  - id: screenshots
    match:
      glob: ["Screenshot*", "Screen Shot*"]
      extensions: [".png"]
    rename:
      mode: snake
```

YAML accepts exactly one document with typed mappings/sequences/scalars. Anchors, aliases, merge keys, custom tags, nulls, and type coercion are rejected. Booleans must be lowercase `true`/`false`; version must be a canonical nonnegative decimal integer. Quote strings that YAML would interpret as numbers, booleans, nulls, or timestamps. Single-quoted strings preserve backslashes and interpolation quotes, e.g. `filename: 'shot_${file.modified | date("timestamp")}_${index | pad(3)}${file.ext | lower}'`. Literal/folded block scalars retain YAML's normal newline behavior; invalid resulting names are not silently repaired (use `|-` if no trailing newline is intended).

Each rule requires a unique 1–64 byte ID starting with an ASCII letter/digit and otherwise containing letters, digits, `_`, `-`, or `.`. A rule requires a rename or move action (or both). When rename exists, supply exactly one of `rename.mode` and `rename.filename`. `rename.mode` supports `upper`, `lower`, `pascal`, `camel`, `snake`, `kebab`, `title`, `screaming`, and `sentence`, preserving the existing mode behavior. `rename.filename` produces the complete basename; see [fields, helpers, escaping, and limits](filename-patterns.md). [Move actions](organization.md) require explicit directory input and history for apply; move-only rules preserve the original basename.

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

The preview prints the effective selection and override names. `--verbose` also shows the chosen rule, action, and per-rule index for each matched item. Dry runs and `template validate` never write undo history. Apply stores a normalized preset snapshot alongside its effective request, so subsequent preset edits cannot change an already-created plan or its undo steps.

## Limits and safety

A `.toml`, `.yaml`, or `.yml` file must be a regular file and contain at most 64 KiB of valid UTF-8. YAML also limits the parsed schema tree to 16 levels and 16,384 nodes before typed decoding. A preset may contain 1–128 rules, at most 32 glob/extension values per rule field and 32 additional ignores, and glob patterns of at most 1,024 bytes. Labels are limited to 128 bytes; extension values to 255 bytes. [Filename patterns have additional bounds](filename-patterns.md#diagnostics-limits-and-safety).

The ordinary [rename/undo safety rules](../readme.md#rename-and-undo-safety) apply. In particular, preview is not a transaction or a promise that the filesystem will remain unchanged until apply; stale sources or newly occupied targets stop execution. Named references freeze the resolved origin path and normalized spec in the plan/history. Undo does not reload templates. History remains in its existing platform-native location, independent of template storage.
