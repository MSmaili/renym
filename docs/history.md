# History

Renym stores rename history locally and uses it to support the `undo` command.

History is written automatically for each rename operation unless `--skip-history` is used.

---

## Storage Location

History files are stored locally per operating system:

| OS      | Location                                    |     |
| ------- | ------------------------------------------- | --- |
| Windows | `%APPDATA%\renym\history`                     |     |
| macOS   | `~/Library/Application Support/renym/history` |     |
| Linux   | `~/.local/share/renym/history`                |     |

---

## Default Behavior

- History is enabled by default.
- History is stored per target directory (path).
- Renym stores up to the last two rename operations per directory.
- History is required for undo functionality.

---

## Skipping History

### Skip history for a single operation

```bash
renym --skip-history
```

This prevents the current rename operation from being recorded.

---

## Notes

- If history is skipped or deleted, undo is not possible for those operations.
- History files are stored in JSON format.
- Renym does not provide a global option to disable history; history control is handled per operation.
- History is stored per target directory (path) and only the last two operations are kept.
- If files or directories were renamed manually after Renym ran, undo may fail for those entries.

---
