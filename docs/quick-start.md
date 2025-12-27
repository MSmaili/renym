# Quick Start

Install RNM and run a first command.

---

## Install

### Windows

1. Download the Windows RNM executable from the Releases page. 
2. Place the executable in a directory included in your system `PATH`. 

Verify:

```bash
rnm --version
```

---

### macOS

1. Download the macOS RNM binary from the Releases page. 
2. Make it executable: 

```bash
chmod +x rnm
```

3. Move it to a directory in your `PATH` (example: `/usr/local/bin`). 

Verify:

```bash
rnm --version
```

---

### Linux

1. Download the Linux RNM binary for your architecture. 
2. Make it executable: 

```bash
chmod +x rnm
```

3. Move it to a directory in your `PATH` (example: `/usr/local/bin`). 

Verify:

```bash
rnm --version
```

---

## Run

Preview a rename in the current directory:

```bash
rnm -m kebab --dry-run
```

Apply the same operation:

```bash
rnm -m kebab
```

---

## Notes

- RNM is distributed as a standalone binary.
- No external dependencies are required.
- RNM does not rename files or directories when `--dry-run` is used.
- Package manager support and GUI distribution are planned but not yet available.

---

## See also

* [Basic Usage](basic-usage.md)
* [Modes](modes.md)
* [CLI Reference](cli-reference.md)

---