# rnm — Fast, Safe, Cross-Platform File Rename Tool

**rnm** is a high-performance, extensible, cross-platform command-line tool for batch renaming files and directories.
It provides a clean architecture, safe rename planning, and a powerful rename _expression language_ that supports metadata, counters, formatting functions, and optional regex captures.

## Why this tool?

This tool mainly is created for my brother. He needed a tool to rename, and silly me said it is easy to create, so here we are.

---

## Features

TODO: for me

the desired config flags:

```go
type Config struct {
	Path            string // location where to rename by default current "."
	Mode            string // how do we want to rename? pascal? snake_case?
	Recursive       bool // recursivly map through directiroes and files
	Files           bool // enable/disable file search
	Directories     bool // enable/disable directory search
	Ignore          []string // way to ignore some files/directories via global pattern
	NoDefaultIgnore bool // we should add deafult ignore patterns for saffety, TODO: probably check some other libs, what makes sense as deafult. currently .git
	DryRun          bool // good thing to not run/exectue command just console.log
}
```
