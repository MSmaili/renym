# CLI Reference

This page defines the authoritative command-line interface for Renym.
All flags, commands, and defaults listed here reflect current CLI behavior.

---

## Usage

| Form                   | Description                                                |
| ---------------------- | ---------------------------------------------------------- |
| `renym [flags]`          | Run a rename operation using flags                         |
| `renym [command]`        | Run a subcommand (`help`, `version`, `undo`, `completion`) |
| `renym [command] --help` | Show help for a specific subcommand                        |

---

## Modes (Summary)

Modes define how filenames are transformed.

This section provides a summary only.  
For detailed behavior rules and directory interactions, see [Modes](modes.md).

| Mode     | Example transformation    |
| -------- | ------------------------- |
| `upper`  | `filename` → `FILENAME`   |
| `lower`  | `FILENAME` → `filename`   |
| `pascal` | `file name` → `FileName`  |
| `camel`  | `file name` → `fileName`  |
| `snake`  | `file name` → `file_name` |
| `kebab`  | `file name` → `file-name` |
| `title`  | `file name` → `File Name` |

---

## Examples

|Command|Description|
|---|---|
|`renym -m upper`|Rename files in the current directory using `upper` mode|
|`renym -m snake -p ./photos`|Rename files in `./photos` using `snake` mode|
|`renym -m kebab --dry-run`|Preview a `kebab` rename without applying changes|

---

## Commands

|Command|Description|
|---|---|
|`completion`|Generate shell autocompletion scripts|
|`help`|Show help for a command|
|`undo`|Undo rename operations using local history|
|`version`|Show installed Renym version|

---

## Flags

|Flag|Type|Default|Description|
|---|--:|--:|---|
|`-d`, `--directories`|bool|`false`|Include directories in rename operations|
|`-D`, `--dirs-only`|bool|`false`|Rename directories only, skip files|
|`-n`, `--dry-run`|bool|`false`|Preview changes without modifying the filesystem|
|`-h`, `--help`|bool|—|Show help for `renym`|
|`--ignore <pattern>`|string (repeatable)|—|Glob pattern to exclude paths from renaming|
|`-m`, `--mode <mode>`|string|—|Rename mode (`upper`, `lower`, `pascal`, `camel`, `snake`, `kebab`, `title`)|
|`--no-default-ignore`|bool|`false`|Disable default ignore patterns (`.git`, `.svn`, `.hg`)|
|`-p`, `--path <path>`|string|`.`|Target file or directory|
|`-r`, `--recursive`|bool|`false`|Process subdirectories recursively|
|`--skip-history`|bool|`false`|Skip recording operation history (disables undo)|
|`-v`, `--version`|bool|—|Show installed version|

---

## Notes

- If conflicting flags are provided, Renym applies deterministic precedence.
- Flags not listed here are not part of the public CLI interface.

---
