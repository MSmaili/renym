# Undo

Undo reverses verified rename and organization operations using locally recorded history.

Undo relies on rename history. If history is missing for an operation, that operation cannot be undone.

See: [History](history.md)

---

## Requirements

Undo works only if:

- History was recorded for the operation (`--skip-history` was not used).
- The relevant history file still exists.
- The target path still exists and can be resolved.

---

## Usage

| Command           | Description                                                    |
| ----------------- | -------------------------------------------------------------- |
| `renym undo`        | Undo the most recent rename operation in the current directory |
| `renym undo --path <folder>` | Undo the latest eligible run for an input folder |
| `renym undo --run <id>` | Select the latest eligible run by ID from `renym history` |
| `renym undo --path <folder> --dry-run` | Preview without changing files or journals |

---

## Notes

- Undo operates only on recorded history.
- Deleting history disables undo for the affected operations.
- History files are stored in JSON format.
- `--path` and `--run` are mutually exclusive; positional paths are not accepted.
- Run-ID selection requires a live original input folder and cannot bypass newer or uncertain runs.
- [Organization undo](organization.md) restores only unchanged files into absent original names, then checks owned empty directories. Previews report cleanup candidates, not promises of removal; populated/replaced directories are preserved. Organization undo audits are retained rather than deleted.
- Safe partial undo can resume after resolving a conflict. Pending/undoing active intent or unverified mutations require reconciliation; never blindly retry them.

---
