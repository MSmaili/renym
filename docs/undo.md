# Undo

Undo reverts rename operations using locally recorded history.

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
| `rnm undo`        | Undo the most recent rename operation in the current directory |
| `rnm undo <path>` | Undo rename operations for a specific path                     |

---

## Notes

- Undo operates only on recorded history.
- Deleting history disables undo for the affected operations.
- History files are stored in JSON format.

---