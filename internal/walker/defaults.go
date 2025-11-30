package walker

var DefaultIgnorePatterns = []string{
	// Version control
	".git",
	".svn",
	".hg",

	// Dependencies
	"node_modules",
	"vendor",

	// Build outputs
	"dist",
	"build",
	"target",

	// Caches
	"__pycache__",
	".cache",

	// Virtual environments
	".venv",
	"venv",

	// OS files
	".DS_Store",
	"Thumbs.db",
}
