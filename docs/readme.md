# RNM

RNM is a cross-platform CLI tool for batch renaming files and directories using consistent casing rules and explicit scope control. 

It supports previewing changes, ignoring paths, and undo via local history. 

RNM is CLI-first. A GUI that mirrors the CLI behavior is planned. 

---

## Features

* Rename files using casing formats such as kebab-case, snake_case, Title Case, camelCase, and PascalCase. 
* Apply rename operations recursively across directory trees. 
* Optionally include directories or target directories only during rename operations. 
* Preview changes using dry run before applying them. 
* Record local history for rename operations to support undo (unless history is skipped for an operation). 
* Ignore rules with sensible defaults. 

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

## Planned

* A minimalist GUI that exposes the same functionality as the CLI. 
* Planned integration with the Windows context menu. 