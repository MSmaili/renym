---
type:
  - document
tags:
  - note
created: 2025-12-16 21:26:09
modified: 2025-12-17 00:52:23
---
# History

RNM stores rename history locally and uses it to support the `undo` command.

History is written automatically for each rename operation unless `--skip-history` is used.

---

## Storage Location

History files are stored locally per operating system:

| OS      | Location                                    |     |
| ------- | ------------------------------------------- | --- |
| Windows | `%APPDATA%\rnm\history`                     |     |
| macOS   | `~/Library/Application Support/rnm/history` |     |
| Linux   | `~/.local/share/rnm/history`                |     |

---

## Default Behavior

- History is enabled by default.
- Each rename operation creates or updates a local history record.
- History is required for undo functionality.

---

## Skipping History

### Skip history for a single operation

```bash
rnm --skip-history
```

This prevents the current rename operation from being recorded.

---

## Notes

- If history is skipped or deleted, undo is not possible for those operations.
- History files are stored in JSON format.
- RNM does not provide a global option to disable history; history control is handled per operation.

---
