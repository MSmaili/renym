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
| Linux   | `~/.config/renym/history`                    |     |

---

## Default Behavior

- History is enabled by default.
- History is stored per target directory (path).
- Ordinary completed rename records are pruned to the last two per directory; uncertain records are preserved.
- Organization schema-2 journals and undo audits are retained indefinitely.
- History is required for undo functionality.

---

## Skipping History

### Skip history for a single operation

```bash
renym --skip-history
```

This prevents the current rename operation from being recorded.
It cannot bypass mandatory history for organization apply.

## Discovering runs

```sh
renym history
renym undo --run <id> --dry-run
```

The listing includes input paths, journal states, recorded/undone file steps,
directory ownership/cleanup counts and active recovery intent. Corrupt journals
remain visible. Discovery does not require the input folder to remain populated
or even exist; undo does require its validated live identity. Pending/undoing
intent must be reconciled manually, never automatically replayed.

Linux uses an absolute `XDG_CONFIG_HOME` when set. macOS uses
`~/Library/Application Support`; Windows uses `%APPDATA%`.

---

## Notes

- If history is skipped or deleted, undo is not possible for those operations.
- History files are stored in JSON format.
- Renym does not provide a global option to disable history; history control is handled per operation.
- History is stored per target directory (path) and only the last two operations are kept.
- If files or directories were renamed manually after Renym ran, undo may fail for those entries.

---
