# Safety Overview

RNM is designed to perform bulk rename operations safely.
Safety is achieved through previewing, scoped execution, local history, and undo support.

This page describes how these features work together.
Detailed behavior is documented on their respective pages.

---

## Core Safety Features

### Dry Run

Dry run allows you to preview rename operations without modifying the filesystem.

- Shows exactly what would be renamed
- Writes no history
- Applies no changes

See: [Dry Run](dry-run.md)

---

### History

RNM records rename operations locally to enable undo functionality.

- Enabled by default
- Stored per operation
- Can be skipped explicitly

See: [History](history.md)

---

### Undo

Undo reverts rename operations using recorded history.

- Works across large rename batches
- Requires history to be present
- Stops working if history is deleted or paths are moved

See: [Undo](undo.md)

---

### Ignore Rules

Ignore rules prevent files or directories from being renamed.

- Glob-based patterns
- Default ignores enabled
- Can be overridden per operation

See: [Ignore Rules](ignore.md)

---

## Recommended Safety Workflow

1. Define ignore rules to limit scope.
2. Run the command with `--dry-run`.
3. Review the output carefully.
4. Run the same command without `--dry-run`.
5. Use `undo` if the result is incorrect.

---

## Summary

RNM avoids irreversible operations by design.
Previewing, history, and undo are first-class features and should be used for all non-trivial rename operations.

---