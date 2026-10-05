# Filename patterns

Use `rename.filename` instead of `rename.mode` to construct a **complete basename**. Extensions are explicit, not automatically appended:

```toml
version = 1

[[rules]]
id = "all"
[rules.rename]
filename = '${file.stem | snake}${file.ext}'
```

```sh
renym template validate ./examples/templates/screenshot-dates.toml
renym --template ./examples/templates/screenshot-dates.toml -p ./inbox --dry-run
```

Supply **exactly one** action per rule: `mode` or `filename`. An empty filename is invalid. [Selection and first-match behavior](templates.md) are unchanged. Unmatched files remain untouched; a matched rule does not fall through to another rule on a no-op, invalid output, or conflict.

## Recipes

```toml
# Normalize the stem but preserve the extension's spelling:
filename = '${file.stem | snake}${file.ext}'

# Lowercase the extension too:
filename = '${file.stem | kebab}${file.ext | lower}'

# UTC modification timestamp and a per-rule ordinal:
filename = 'shot_${file.modified | date("timestamp")}_${index | pad(3)}${file.ext | lower}'

# Prefix without changing the original name:
filename = 'archive_${file.name}'
```

Each recipe is an alternative `filename` value, not multiple keys in one rule. TOML single-quoted literal strings avoid double-escaping the expression's JSON quotes/backslashes.

## Original context

| Field | Type | Value |
| --- | --- | --- |
| `file.name` | string | Original complete basename |
| `file.stem` | string | Original basename minus its last extension; full directory name |
| `file.ext` | string | Original last extension including its dot, or empty; directories always empty |
| `file.modified` | timestamp | Original snapshot modification time; requires `date(...)` |
| `file.size` | integer | Regular-file bytes; unavailable for directories |
| `index` | integer | 1-based per-rule ordinal for this plan |

`.env` has stem `.env` and no extension. `.local.env` has stem `.local` and extension `.env`. `archive.tar.gz` has stem `archive.tar` and extension `.gz`. A directory `project.v1` has stem `project.v1` and no extension. `${file.stem}${file.ext}` therefore preserves these names exactly; an explicit normalization helper may change them.

Indices are assigned in **lexical relative-path order**, independently for each first-matched rule, **before** no-op, render-error, or collision skips. Unmatched/protected/unsupported entries do not count. Skips may leave gaps; later items are not renumbered. Nested directories still execute deepest-first, not in index order. Indices restart with each plan and are **not persistent counters**; indexed patterns may change names again on another run. Preview first.

## Helpers and types

| Helper | Input → output | Meaning |
| --- | --- | --- |
| `snake`, `kebab`, `pascal`, `camel`, `title`, `screaming`, `sentence` | string → string | Existing word normalization behavior; no implicit filename sanitization |
| `lower`, `upper` | string → string | Unicode case conversion **preserving punctuation**, unlike existing normalizing lower/upper modes |
| `pad(width)` | integer → string | Zero-pad to width 1–16, without truncating a longer number |
| `date("preset")` | timestamp → string | Format using the UTC presets below |

Date presets:

| Preset | Output shape |
| --- | --- |
| `"date"` | `2026-10-05` |
| `"month"` | `2026-10` |
| `"year"` | `2026` |
| `"timestamp"` | `2026-10-05_142530` |

These are preset names, **not** Go layouts, strftime, or Moment tokens. UTC can produce a different date from your local timezone. No birth-time fallback, current-clock field, or configurable timezone is provided.

String helpers have no arguments or parentheses (`| snake`, not `| snake()`). `pad` and `date` require parentheses and exactly one literal argument. Pipeline types must agree; `${file.stem | pad(3)}` and `${file.modified | lower}` are configuration errors. Strings and integers may be emitted directly; timestamps must be formatted. Missing requested metadata is an item error, never an empty string or made-up value.

## Syntax and escaping

```text
${ field | helper | helper(literal) }
```

- Only the fields/helpers above are allowed. ASCII whitespace inside expressions is optional.
- Arguments are JSON double-quoted strings or canonical nonnegative decimal integers: `0`, `1`, `16`. No signs, leading zeros (`010`), fractions, exponents, or values above `9223372036854775807`.
- String escaping follows JSON, including valid `\uXXXX` escapes and paired Unicode surrogates. Invalid UTF-8/unpaired surrogates are rejected, not silently replaced. Quotes, `|`, and `}` inside quoted arguments do not end the expression.
- `$$` emits a literal `$`: `$${file.stem}` produces the literal text `${file.stem}`. Other dollar signs are literal unless followed by `{`. Malformed `${...}` is an error.
- Original names and helper output are **never parsed again**. A file named `${index}.txt` stays literal when emitted through `${file.name}`.
- No arithmetic, predicates, conditions, loops, definitions, nested expressions, methods, regex replacement, shell/environment expansion, filesystem/network access, randomness, or AI calls. `{{...}}` is ordinary literal text, not a second template language.

## Diagnostics, limits, and safety

Configuration errors fail before discovering input files. Diagnostics include the template path, rule/field, and a **1-based UTF-8 byte offset in the decoded filename string**. That offset is not a TOML source column: escaping and multiline strings can change positions. TOML parser errors keep source locations where available.

Per-item render errors (missing metadata, invalid output, limits) are reported as skipped with no partial output or fallback rule. Filename output is never sanitized or repaired. It must be one basename: nonempty, not `.`/`..`, valid UTF-8, no `/`, `\`, or control characters, and at most **255 bytes**. The shared planner additionally enforces target-platform rules such as Windows reserved names/characters, and protects existing targets without inventing suffixes.

Per filename: **4 KiB source**, **64 parts** (literal runs/interpolations), **16 pipeline helpers per interpolation**, **128 lexical tokens per interpolation**, and **1 KiB** decoded string arguments, requested name context, and intermediate values. Decimal values use nonnegative signed 64-bit range. Existing 64 KiB document/128-rule limits still apply.

Rendering is pure: the application supplies only requested metadata from its original safety snapshots, without additional reads for renderer-only fields. An already-created plan freezes its targets and preset snapshot; apply/undo never re-evaluate expressions. Existing [execution/history safety limitations](../readme.md#rename-and-undo-safety) still apply. YAML, named lookup, moves, watching, and classification are not part of this slice.
