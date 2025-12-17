---
type:
  - document
tags:
  - note
created: 2025-12-16 22:32:18
modified: 2025-12-17 00:36:47
---
# Installation

Install and run RNM on supported platforms.

---

## Windows

1. Download the Windows RNM executable from the Releases page.
2. Place the executable in a directory included in your system `PATH`.

### Verify installation

```bash
rnm --version
```

If a version is printed, RNM is installed correctly.

---

## macOS

1. Download the macOS RNM binary from the Releases page.
2. Make the binary executable:

```bash
chmod +x rnm
```

1. Move it to a directory in your `PATH` (for example `/usr/local/bin`).

### Verify installation

```bash
rnm --version
```

---

## Linux

1. Download the Linux RNM binary for your architecture.
2. Make the binary executable:

```bash
chmod +x rnm
```

1. Move it to a directory in your `PATH` (for example `/usr/local/bin`).

### Verify installation

```bash
rnm --version
```

---

## Notes

- RNM is distributed as a standalone binary.
- No external dependencies are required.
- RNM does not rename files or directories when `--dry-run` is used.
- Package manager support and GUI distribution are planned but not yet available.

---
