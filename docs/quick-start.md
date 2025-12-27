# Quick Start

Install Renym and run a first command.

---

## Install

### Windows

1. Download the Windows Renym executable from the Releases page. 
2. Place the executable in a directory included in your system `PATH`. 

Verify:

```bash
renym --version
```

---

### macOS

1. Download the macOS Renym binary from the Releases page. 
2. Make it executable: 

```bash
chmod +x renym
```

3. Move it to a directory in your `PATH` (example: `/usr/local/bin`). 

Verify:

```bash
renym --version
```

---

### Linux

1. Download the Linux Renym binary for your architecture. 
2. Make it executable: 

```bash
chmod +x renym
```

3. Move it to a directory in your `PATH` (example: `/usr/local/bin`). 

Verify:

```bash
renym --version
```

---

## Run

Preview a rename in the current directory:

```bash
renym -m kebab --dry-run
```

Apply the same operation:

```bash
renym -m kebab
```

---

## Notes

- Renym is distributed as a standalone binary.
- No external dependencies are required.
- Renym does not rename files or directories when `--dry-run` is used.
- Package manager support and GUI distribution are planned but not yet available.

---

## See also

* [Basic Usage](basic-usage.md)
* [Modes](modes.md)
* [CLI Reference](cli-reference.md)

---