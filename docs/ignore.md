# Ignore Rules

RNM supports ignore rules to exclude files or directories from rename operations. Ignore rules limit scope and reduce unintended changes.

---

## Default Ignore Rules

RNM ships with a built-in default ignore list.

Examples include common version control and tooling directories such as:

- `.git`
- `.svn`
- `.hg`

The complete list is defined in:

`internal/walker/defaults.go`

To disable default ignores:

```bash
rnm --no-default-ignore
```

---

## Ignoring Paths with Patterns

Use `--ignore` to exclude files or directories matching a glob pattern:

```bash
rnm --ignore "*.tmp"
```

You can use `--ignore` multiple times:

```bash
rnm --ignore "*.tmp" --ignore "folder-name"
```

---

## Examples

|Command|Description|
|---|---|
|`rnm --ignore "*.log"`|Ignore all `.log` files|
|`rnm --ignore "node_modules" -r`|Ignore `node_modules` during recursive rename|
|`rnm --no-default-ignore`|Include VCS directories in rename|

---

## Notes

- Ignore rules are evaluated before rename operations.
- Ignored paths are skipped and not processed for renaming.
- RNM does not currently support ignore files (for example, `.rnmignore`).
- Ignore rules must be provided via CLI flags.

---

## See also

- [Safety Overview](safety.md)
- [Dry Run](dry-run.md)

---
