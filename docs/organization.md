# Organization previews — experimental, preview only

Templates describe reusable rules and per-rule destinations; the invocation
selects the input folder. **Moving and
move undo remain gated pending native CLI acceptance.** Any template containing a move rule requires
`--dry-run`; without it, the entire workflow fails before discovery/mutation,
including rename-only rules in that same template. `--skip-history` cannot bypass
this gate. The journaled workflow is integrated for acceptance testing, but ordinary
builds remain read-only for moves. Watching and later phases are still deferred.

```sh
renym template validate ./examples/templates/organization-preview.toml
renym --path ~/Downloads --template ./examples/templates/organization-preview.toml --dry-run
# Equivalent YAML:
renym --path ~/Downloads --template ./examples/templates/organization-preview.yaml --dry-run
# Named lookup also works after saving the template in configured storage:
renym --path ~/Downloads --template organize-downloads --dry-run
```

The invocation reads `~/Downloads`; the example proposes screenshots beneath
`~/Pictures/Renym/screenshots/YYYY-MM/`. Choose any existing input folder with
`--path` and edit the template's destination to your intended output location.
The same template can be used for several input folders without editing it.
Nothing creates output folders or writes history
in preview. Validation checks schema/patterns without requiring existing roots.

## Schema additions

```toml
version = 1
[[rules]]
id = "documents"
[rules.match]
extensions = [".pdf"]
[rules.move]
root = "~/Documents/Sorted"
directory = 'documents/${file.modified | date("month")}' # optional
# Omit rename to preserve the complete original basename.
# Add [rules.rename] with mode or filename to combine rename and move.
```

- Templates do not contain `source`; it is rejected as an unknown field in both
  formats. Organization requires an explicit `--path` selecting an existing
  directory. Relative paths and an explicit `--path .` are supported; omitting
  the input is an error rather than silently organizing cwd. Ordinary rename-only
  templates retain their existing explicit-path/current-directory behavior and
  safe apply/undo. Shell expansion handles `~` in CLI paths.
- A rule requires at least one of `rename` or `move`. When `rename` exists,
  exactly one of `mode` or `filename` is required. Move-only preserves the name;
  combining actions produces one target proposal, not a rename then a move.
- Every move supplies its own literal `root`; multiple roots are supported.
  Move templates select regular files only, including after CLI overrides;
  folder relocation is unsupported. Recursion remains an explicit selection.
- Roots must be native absolute paths or `~/...`. Only this home prefix is
  expanded; no `~user`, environment expansion, interpolation, relative roots,
  dot segments, UNC, or device namespace paths. Root strings are bounded to
  4,096 UTF-8 bytes. Paths do not depend on the template directory or cwd.
  Ordinary aliases of existing ancestor paths are resolved for previews; the
  root itself cannot be a symlink. Source and destination roots must be disjoint
  (including containment in either direction), conservatively ignoring case.
- Omit `directory` to target the root itself. Otherwise use literal `/` separators
  between independently rendered components. Each component supports the existing
  filename field/helper grammar; values cannot introduce extra separators.
  Absolute/drive paths, empty/dot components, backslashes, control characters,
  Windows-reserved names, trailing dots/spaces and illegal characters fail. No
  implicit repair/sanitization. Maximum: 16 components, 255 UTF-8 bytes each,
  1,024 bytes total output, and a 4,096-byte source pattern. Errors identify rule,
  component, and the existing 1-based decoded filename byte offset **within that
  component**, not a fabricated file column.
- First-match rules, original snapshots, UTC modification times, per-rule lexical
  indices (including skipped matches), and no fallback remain unchanged.

## Preview safeguards and remaining work

Previews report occupied/dangling targets, duplicate batch targets, and file versus
directory namespace conflicts across roots. Sources remain untouched. Accepted
targets determine the displayed, deduplicated directory-creation list. Known
source/destination filesystem identity mismatches and mount crossings are skipped;
linked/non-directory destination descendants are rejected. Active template/history
paths remain protected, including ordinary aliases of not-yet-created history paths.

**Read-only checks do not guarantee a later mutation will succeed.** The integrated
workflow pins and revalidates root/parent handles, uses native rooted no-replace
moves, and journals directory ownership and verified completion. Public mutation
remains gated until the integrated workflow passes native CLI acceptance.

## Journals and undo

`renym history` discovers saved runs by stable ID, even when the input folder is
empty or missing. `renym undo --path <original-folder> --dry-run` and
`renym undo --run <id> --dry-run` preview eligible undo steps and owned-directory
cleanup checks. Run-ID selection cannot skip a newer run or uncertain intent;
actual undo still requires a live, unchanged original input folder.

Organization uses schema-2 journals, distinct from schema-1 rename history.
These records are retained indefinitely, including successful undo audits and
reasons for preserving non-empty, missing or replaced directories. Only
verified owned empty directories are eligible for removal, after file reversal.
There is no automatic reconciliation: attempted unverified mutations or failed
checkpoints block automatic replay. Never blindly retry such a run.

The underlying workflow supports trusted local directories and one writer,
not hostile namespace changes, atomic batches or cross-volume copy/delete.
Snapshots compare identity, mode, size and modification time, not content hashes.
