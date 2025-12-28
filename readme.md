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
