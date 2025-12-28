package main

import (
	"os"

	"github.com/MSmaili/renym/internal/cli"
	"github.com/MSmaili/renym/internal/log"
	"github.com/spf13/cobra"
)

// GlobalConfig holds flags shared across all commands
type GlobalConfig struct {
	DryRun  bool
	Verbose bool
	Quiet   bool
}

var globalCfg GlobalConfig

var rootCmd = &cobra.Command{
	Use:   "renym [flags]",
	Short: "Fast, safe, cross-platform file rename tool",
	Long: `Rename files and directories using automatic naming patterns.

Modes:
  upper   → FILENAME
  lower   → filename
  pascal  → FileName
  camel   → fileName
  snake   → file_name
  kebab   → file-name
  title   → File Name`,
	Example: `
  renym -m upper
  renym -m snake -p ./photos
  renym -m kebab --dry-run
  renym -m snake -v          # verbose output
  renym -m snake -q          # quiet mode`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return setupLogging()
	},
	PreRunE: validateFlags,
	RunE:    runRename,
}

func init() {
	initGlobalFlags(rootCmd)
}

// initGlobalFlags registers persistent flags that are inherited by all commands
func initGlobalFlags(cmd *cobra.Command) {
	pf := cmd.PersistentFlags()
	pf.BoolVarP(&globalCfg.DryRun, "dry-run", "n", false, "Preview changes without executing")
	pf.BoolVarP(&globalCfg.Verbose, "verbose", "v", false, "Enable verbose output")
	pf.BoolVarP(&globalCfg.Quiet, "quiet", "q", false, "Suppress non-essential output")
}

// hideGlobalFlags hides global flags from a command's help output.
func hideGlobalFlags(cmd *cobra.Command) {
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		c.Flags().MarkHidden("dry-run")
		c.Flags().MarkHidden("verbose")
		c.Flags().MarkHidden("quiet")
		c.Parent().HelpFunc()(c, args)
	})
}

// setupLogging configures log level based on flags.
func setupLogging() error {
	if err := cli.ValidateGlobalFlags(globalCfg.Verbose, globalCfg.Quiet); err != nil {
		return err
	}

	switch {
	case globalCfg.Quiet:
		log.SetLevel(log.LevelSilent)
	case globalCfg.Verbose:
		log.SetLevel(log.LevelDebug)
	default:
		log.SetLevel(log.LevelNormal)
	}
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Error("%v\n", err)
		os.Exit(1)
	}
}
