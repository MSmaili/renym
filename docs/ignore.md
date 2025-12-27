# Ignore Rules

Renym supports ignore rules to exclude files or directories from rename operations. Ignore rules limit scope and reduce unintended changes.

---

## Default Ignore Rules

Renym ships with a built-in default ignore list.

Examples include common version control and tooling directories such as:

- `.git`
- `.svn`
- `.hg`

The complete list is defined in:

`internal/walker/defaults.go`

To disable default ignores:

```bash
renym --no-default-ignore
```

---

## Ignoring Paths with Patterns

Use `--ignore` to exclude files or directories matching a glob pattern:

```bash
renym --ignore "*.tmp"
```

You can use `--ignore` multiple times:

```bash
renym --ignore "*.tmp" --ignore "folder-name"
```

---

## Examples

|Command|Description|
|---|---|
|`renym --ignore "*.log"`|Ignore all `.log` files|
|`renym --ignore "node_modules" -r`|Ignore `node_modules` during recursive rename|
|`renym --no-default-ignore`|Include VCS directories in rename|

---

## Notes

- Ignore rules are evaluated before rename operations.
- Ignored paths are skipped and not processed for renaming.
- Renym does not currently support ignore files (for example, `.renymignore`).
- Ignore rules must be provided via CLI flags.

---

## See also

- [Safety Overview](safety.md)
- [Dry Run](dry-run.md)

---
