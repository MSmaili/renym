# Basic Usage

Renym renames files and directories by applying a rename mode to a target path, with optional scope and safety controls.

---

## Core Command Structure

```bash
renym --mode <mode> [flags]
```

A rename operation consists of:

1. A rename mode
2. A target path
3. Optional scope and safety flags

---

## Target Path

Renym operates on the current working directory by default.

```bash
renym -m kebab
```

To target a specific path:

```bash
renym -m kebab --path ./photos
```

The path may point to:

- a directory
- a single file

---

## Scope

### Recursive operation

By default, Renym processes only the immediate contents of the target directory.

To include subdirectories:

```bash
renym -m snake --recursive
```

---

### Directory handling

By default, Renym renames files only.

To include directories:

```bash
renym -m kebab --directories
```

To rename only directories:

```bash
renym -m kebab --dirs-only
```

---

## Ignore Rules

Paths can be excluded using ignore patterns.

```bash
renym --ignore "*.tmp" --ignore "node_modules"
```

Ignore rules are applied before rename logic.

Default ignore patterns exclude common version control directories. 

See: [Ignore Rules](ignore.md)

---

## Safety Controls

Renym provides mechanisms to limit unintended changes.

- Preview changes using `--dry-run`
- Revert operations using `renym undo`
- Skip history recording using `--skip-history`

See: [Safety Overview](safety.md)

---

## Execution Order

Renym processes operations in the following order:

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

- [Quick Start](quick-start.md)
- [Modes](modes.md)
- [CLI Reference](cli-reference.md)
- [Safety Overview](safety.md)

---
