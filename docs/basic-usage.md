---
type:
  - document
tags:
  - note
created: 2025-12-16 22:25:05
modified: 2025-12-17 00:48:06
---
# Basic Usage

RNM renames files and directories by applying a rename mode to a target path, with optional scope and safety controls.

---

## Core Command Structure

```bash
rnm --mode <mode> [flags]
```

A rename operation consists of:

1. A rename mode
2. A target path
3. Optional scope and safety flags

---

## Target Path

RNM operates on the current working directory by default.

```bash
rnm -m kebab
```

To target a specific path:

```bash
rnm -m kebab --path ./photos
```

The path may point to:

- a directory
- a single file

---

## Scope

### Recursive operation

By default, RNM processes only the immediate contents of the target directory.

To include subdirectories:

```bash
rnm -m snake --recursive
```

---

### Directory handling

By default, RNM renames files only.

To include directories:

```bash
rnm -m kebab --directories
```

To rename only directories:

```bash
rnm -m kebab --dirs-only
```

---

## Ignore Rules

Paths can be excluded using ignore patterns.

```bash
rnm --ignore "*.tmp" --ignore "node_modules"
```

Ignore rules are applied before rename logic.

Default ignore patterns exclude common version control directories. See: [Ignore Rules](ignore.md)

---

## Safety Controls

RNM provides mechanisms to limit unintended changes.

- Preview changes using `--dry-run`
- Revert operations using `rnm undo`
- Skip history recording using `--skip-history`

See: [Safety Overview](safety.md)

---

## Execution Order

RNM processes operations in the following order:

1. Resolve target path
2. Apply ignore rules
3. Determine scope (files, directories, recursion)
4. Apply rename mode
5. Record history (unless `--skip-history` is used)

---

## Notes

- Rename modes do not affect file contents.
- File extensions are preserved and normalized separately.
- Large or recursive operations should be reviewed before execution.

---

## See also

- [Modes](modes.md)
- [Quick Start](quick-start.md)
- [CLI Reference](cli-reference.md)
- [Safety Overview](safety.md)

---
