package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "studio",
	Short: "AgentStudio — autonomous GitHub agent controller",
	Long: `studio controls AgentStudio: a system that runs headless Claude sessions
on a cron schedule to triage GitHub issues and open pull requests autonomously.

All state is stored in ~/.agent-studio/ (override with AGENT_STUDIO_HOME).

Get started:
  studio install        # set up ~/.agent-studio/ and register cron jobs
  studio jobs list      # view scheduled jobs and their status
  studio config show    # view current configuration`,
	Version: version,
}

// SetVersion injects the build-time version string.
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(jobsCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(queueCmd)
	rootCmd.AddCommand(promptsCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(diffCmd)
}
