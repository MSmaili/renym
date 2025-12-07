# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [v0.0.3] - 2025-12-07

### Added

- Version command to display the current version
  -- We added also a flag for version, -V and --version
- Undo command that reads history and reverts rename operations
- History tracking for rename operations
- Support for renaming a single file
- Small readme refactoring for installation guide

### Fixed

- Sanitize text before splitting words
- Pass skipHistory to proper flag

### Changed

- Add config to the history file

## [v0.0.2] - 2025-12-06

### Added

- Default sanitize functionality for filenames
- Different word splitting strategies: lowercase to uppercase, drop acronym
- Improved help and information when running commands

### Fixed

- Main directory path handling

### Changed

- Removed logging for skipping files (cleaner output)

## [v0.0.1] - 2025-11-30

### Added

- Initial release
- Directory walker with path support and extra flags
- Basic file rename functionality for Unix systems
- Windows file adapter support
- Cobra CLI with simple rename command
- Collision and duplication checks for existing files
- Dry-run flag for previewing changes
- Validation for modes and flags
- Extra rename modes with rune support
- Case sensitivity checks for Mac, Linux, and Windows
- Default ignores for safer renaming with `--no-defaults` option to skip
- GoReleaser configuration for releases
- Common slice mapper for mapping between types
- Simpler logging implementation

### Fixed

- Order directories by depth when renaming directories
- Lower/upper modes state preservation
- Case sensitive check for Windows
- Main directory path issues
- Missing GoReleaser configuration

[Unreleased]: https://github.com/MSmaili/rnm/compare/v0.0.2...HEAD
[v0.0.2]: https://github.com/MSmaili/rnm/compare/v0.0.1...v0.0.2
[v0.0.1]: https://github.com/MSmaili/rnm/releases/tag/v0.0.1
