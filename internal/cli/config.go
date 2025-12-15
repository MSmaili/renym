package cli

type Config struct {
	Path            string
	Mode            string
	Recursive       bool
	Directories     bool
	Files           bool
	Ignore          []string
	NoDefaultIgnore bool
	DryRun          bool
	SkipHistory     bool
}
