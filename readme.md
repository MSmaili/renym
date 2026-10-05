# renym

A fast, safe, cross-platform file rename tool.

## Install

### Go

```bash
go install github.com/MSmaili/renym@latest
```

### Binary

Download the latest release from [GitHub Releases](https://github.com/MSmaili/renym/releases/latest).

#### Windows

TODO: probably we need to improve this and add the package in winget...

1. Download `renym_<version>_windows_amd64.zip` (or `arm64` for ARM devices)
2. Extract the zip file to a folder, e.g., `C:\Program Files\renym\`
3. Add to PATH:
   - Open Start Menu, search **"Environment Variables"**
   - Click **"Edit the system environment variables"**
   - Click **"Environment Variables..."**
   - Under **"User variables"**, select **Path** → **Edit** → **New**
   - Add the folder path: `C:\Program Files\renym`
   - Click **OK** to save
4. Open a new terminal and verify: `renym --version`

#### macOS / Linux

```bash
# Download (replace <version> and <os>/<arch> as needed)
curl -LO https://github.com/MSmaili/renym/releases/latest/download/renym_<version>_<os>_<arch>.tar.gz

# Extract and install to ~/.local/bin
tar -xzf renym_*.tar.gz
mkdir -p ~/.local/bin
mv renym ~/.local/bin/

# Add to PATH if already is not there (add this to your ~/.bashrc or ~/.zshrc)
export PATH="$HOME/.local/bin:$PATH"
```

Available archives:

- macOS Intel: `darwin_amd64`
- macOS Apple Silicon: `darwin_arm64`
- Linux: `linux_amd64` or `linux_arm64`

Verify installation: `renym --version`

## Quick Start

```bash
# Convert filenames to snake_case
renym -m snake -p ./photos

# Preview changes first (dry-run)
renym -m kebab -p ./documents --dry-run

# Rename recursively
renym -m pascal -p ./src -r

# Undo last rename
renym undo
```

## Reusable TOML/YAML templates

```bash
renym template validate ./examples/templates/screenshots.toml
renym --template ./examples/templates/screenshots.toml -p ./inbox --dry-run
renym --template ./examples/templates/screenshots.toml -p ./inbox
# Equivalent YAML:
renym template validate ./examples/templates/screenshots.yaml
renym --template ./examples/templates/screenshots.yaml -p ./inbox --dry-run
# After saving screenshots.toml or screenshots.yaml in your template directory:
renym template list
renym --template screenshots -p ./inbox --dry-run
```

Use ordered rules with existing rename modes or [bounded filename patterns](docs/filename-patterns.md). [Schema, storage, matching, and CLI overrides](docs/templates.md). `--template` and `--mode` are exclusive. Named templates live in `~/.config/renym/templates` on macOS/Linux (absolute `XDG_CONFIG_HOME` overrides), or `%APPDATA%\renym\templates` on Windows. Moving, watching, and AI actions are not implemented yet.

## Rename and undo safety

- Previews do not change files or create undo history.
- Occupied targets, including dangling links and batch swaps/chains, are rejected. Native rename calls never overwrite; unsupported filesystems fail instead of using an unsafe fallback.
- Source identity/mode and regular-file size/modification time are rechecked before execution and undo. Detected changes stop the operation. This is not content hashing: same-size edits that retain the same timestamp can go undetected, including writes within one filesystem timestamp tick.
- History records completed physical steps, including partial runs. Undo reverses those steps in order, preserves occupied restore targets, and can resume a recorded partial undo.
- History must be saved before mutation. `--skip-history` is the explicit exception; those changes cannot be undone through Renym.
- When history is enabled, its directory and ancestors are protected from renaming so a run cannot move its own journal.
- Older history files contain unverified plans and cannot be automatically undone by this version. They are not migrated. Interrupted or failed checkpoints also require manual reconciliation rather than blind replay.
- Case-only renames are currently skipped when the destination aliases the source. Batch target comparison is conservative when volume case behavior is unknown.

Use trusted local directories and one Renym operation at a time. This is not an atomic batch transaction or a hostile-filesystem sandbox. Cross-process locking and full crash/power-loss recovery are not implemented.

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

TODO:

MIT
