---
type:
  - document
tags:
  - note
created: 2025-12-16 22:04:40
modified: 2025-12-17 00:51:47
---
# Quick Start

Minimum steps to run RNM

---

## Run RNM

RNM operates on the current directory by default.

```bash
rnm -m kebab
```

This renames files in the current directory using `kebab` mode.

---

## Previewing changes

Preview a rename operation using `--dry-run`:

```bash
rnm -m kebab --dry-run
```

See: [Dry Run](dry-run.md)

---

## Specify a path

Use `--path` to target a specific directory.

```bash
rnm -m snake --path ./photos
```

If `--path` is omitted, the current working directory is used.

---

## Recursive operation

Use `--recursive` to rename files in subdirectories.

```bash
rnm -m camel --path ./src --recursive
```

---

## Directory renaming

Include directories in rename operations:

```bash
rnm -m title --directories
```

Rename only directories:

```bash
rnm -m title --dirs-only
```

---

## Ignoring paths

Exclude files or directories using glob patterns:

```bash
rnm -m kebab --ignore "*.tmp" --ignore "node_modules"
```

See: [Ignore Rules](ignore.md)

---

## Undo

Revert the most recent rename operation:

```bash
rnm undo
```

Undo uses recorded history.

See: [History](history.md) and [Undo](undo.md)

---

## Notes

- `--dry-run` is recommended for recursive or large-scale operations.
- Ignore rules help prevent unintended renames.
- If history is skipped or deleted, undo is not possible for those operations.

---

## See also

- [Basic Usage](basic-usage.md)
- [Modes](modes.md)
- [CLI Reference](cli-reference.md)
- [Safety Overview](safety.md)

---
