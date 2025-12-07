# rnm

A fast, safe, cross-platform file rename tool.

## Install

### Go

```bash
go install github.com/MSmaili/rnm@latest
```

### Binary

Download from [Releases](https://github.com/MSmaili/rnm/releases)

## Quick Start

```bash
# Convert filenames to snake_case
rnm -m snake -p ./photos

# Preview changes first (dry-run)
rnm -m kebab -p ./documents --dry-run

# Rename recursively
rnm -m pascal -p ./src -r

# Undo last rename
rnm undo
```

## Modes

| Mode     | Example     |
| -------- | ----------- |
| `upper`  | `FILENAME`  |
| `lower`  | `filename`  |
| `pascal` | `FileName`  |
| `camel`  | `fileName`  |
| `snake`  | `file_name` |
| `kebab`  | `file-name` |
| `title`  | `File Name` |

## Documentation

Full documentation available at TODO: add /docs and should be avaiable via https://docsify.js.org/

## License

MIT
