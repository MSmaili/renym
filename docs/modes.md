---
type:
  - document
tags:
  - note
created: 2025-12-16 22:23:14
modified: 2025-12-17 00:49:04
---
# Modes

Modes define how RNM transforms file and directory names.

- By default, modes apply to filenames only.
- When directory renaming is enabled, the same transformation rules apply to directory names.
- File extensions are preserved and normalized separately.

---

## Available Modes

| Mode     | Output format | Example                           |
| -------- | ------------- | --------------------------------- |
| `upper`  | Uppercase     | `file name.txt` → `FILE NAME.txt` |
| `lower`  | Lowercase     | `FILE NAME.txt` → `file name.txt` |
| `pascal` | PascalCase    | `file name.txt` → `FileName.txt`  |
| `camel`  | camelCase     | `file name.txt` → `fileName.txt`  |
| `snake`  | snake_case    | `file name.txt` → `file_name.txt` |
| `kebab`  | kebab-case    | `file name.txt` → `file-name.txt` |
| `title`  | Title Case    | `file name.txt` → `File Name.txt` |

---

## Mode Selection

Modes are selected using the `--mode` (`-m`) flag.

```bash
rnm --mode kebab
```

Short form:

```bash
rnm -m kebab
```

---

## Behavior Rules

- Modes are applied to the name portion of the path being renamed.
- The file extension is preserved.
- Extensions are normalized to lowercase.
- Directory names follow the same rules when directory renaming is enabled.

---

## Interaction with Directories

By default, modes apply to files only.

To include directories:

```bash
rnm -m kebab --directories
```

To rename only directories:

```bash
rnm -m kebab --dirs-only
```

---

## Examples

|Command|Result|
|---|---|
|`rnm -m kebab`|Rename files in the current directory|
|`rnm -m snake -r`|Rename files recursively|
|`rnm -m title --directories`|Rename files and directories|

---

## Notes

- Modes do not modify file contents.
- If a rename results in a conflicting name, RNM may skip the item or fail the operation depending on the conflict and platform behavior.
- Use `--dry-run` to preview results.

---

## See also

- [Quick Start](quick-start.md)
- [CLI Reference](cli-reference.md)
- [Safety Overview](safety.md)
- [Ignore Rules](ignore.md)

---
