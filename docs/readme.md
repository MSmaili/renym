---
type:
  - document
tags:
  - note
created: 2025-12-16 22:26:32
modified: 2025-12-17 01:13:19
---
# RNM

RNM is a cross-platform CLI tool for batch renaming files and directories using predefined rename modes and explicit scope control.

It supports previewing changes, ignoring paths, and undo via local history.

---

## Features

- Multiple rename modes (`upper`, `lower`, `pascal`, `camel`, `snake`, `kebab`, `title`)
- Recursive processing and directory renaming
- Dry-run previews and undo support
- Ignore rules with sensible defaults

---

## Example

Preview a rename:

```bash
rnm -m kebab --dry-run
```

Apply it:

```bash
rnm -m kebab
```

Undo the last operation:

```bash
rnm undo
```

---

## Documentation

Full documentation is available:

- [Index](https://github.com/MSmaili/smaili/blob/main/projects/rnm-rename-tool/documentation/index.md)
- [Installation](https://github.com/MSmaili/smaili/blob/main/projects/rnm-rename-tool/documentation/installation.md)
- [Quick Start](https://github.com/MSmaili/smaili/blob/main/projects/rnm-rename-tool/documentation/quick-start.md)
- [Basic Usage](https://github.com/MSmaili/smaili/blob/main/projects/rnm-rename-tool/documentation/basic-usage.md)
- [Modes](https://github.com/MSmaili/smaili/blob/main/projects/rnm-rename-tool/documentation/modes.md)
- [CLI Reference](https://github.com/MSmaili/smaili/blob/main/projects/rnm-rename-tool/documentation/cli-reference.md)
- [Safety Overview](https://github.com/MSmaili/smaili/blob/main/projects/rnm-rename-tool/documentation/safety.md)
    - [Dry Run](https://github.com/MSmaili/smaili/blob/main/projects/rnm-rename-tool/documentation/dry-run.md)
    - [Ignore Rules](https://github.com/MSmaili/smaili/blob/main/projects/rnm-rename-tool/documentation/ignore.md)
    - [History](https://github.com/MSmaili/smaili/blob/main/projects/rnm-rename-tool/documentation/history.md)
    - [Undo](https://github.com/MSmaili/smaili/blob/main/projects/rnm-rename-tool/documentation/undo.md)

---

## Status

RNM is CLI-first. A GUI that mirrors the CLI behavior is planned.

---