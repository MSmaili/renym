---
type:
  - document
tags:
  - note
created: 2025-12-16 21:21:40
modified: 2025-12-17 00:52:23
---

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

| Command             | Description                                                    |
| ------------------- | -------------------------------------------------------------- |
| `renym undo`        | Undo the most recent rename operation in the current directory |
| `renym undo <path>` | Undo rename operations for a specific path                     |

---

## Notes

- Undo operates only on recorded history.
- Deleting history disables undo for the affected operations.
- History files are stored in JSON format.

---

