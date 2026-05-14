package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/maximilianbuff/agent-studio/internal/crontab"
	"github.com/maximilianbuff/agent-studio/internal/home"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove AgentStudio cron jobs and optionally delete the state directory",
	Long: `studio uninstall removes AgentStudio from this machine:

  1. Removes the AgentStudio block from the user crontab
  2. With --purge: deletes ~/.agent-studio/ entirely

The studio binary itself is removed with: make uninstall`,
	RunE: runUninstall,
}

func init() {
	uninstallCmd.Flags().Bool("purge", false, "Also delete ~/.agent-studio/ and all its contents")
	uninstallCmd.Flags().Bool("yes", false, "Skip confirmation prompt (for scripts)")
}

func runUninstall(cmd *cobra.Command, _ []string) error {
	purge, _ := cmd.Flags().GetBool("purge")
	yes, _ := cmd.Flags().GetBool("yes")
	out := cmd.OutOrStdout()
	studioHome := home.Dir()

	step(out, "Removing cron jobs")
	if err := crontab.DeleteBlock(crontab.DefaultRunCmd, crontab.DefaultRunCmdStdin); err != nil {
		return fmt.Errorf("removing crontab block: %w", err)
	}
	line(out, "ok      AgentStudio crontab block removed")

	if purge {
		if !yes {
			fmt.Fprintf(out, "\nThis will permanently delete %s and all its contents.\n", studioHome)
			fmt.Fprint(out, "Type 'yes' to confirm: ")

			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			answer := strings.TrimSpace(scanner.Text())
			if answer != "yes" {
				return fmt.Errorf("aborted")
			}
		}

		step(out, "Deleting %s", studioHome)
		if err := os.RemoveAll(studioHome); err != nil {
			return fmt.Errorf("deleting %s: %w", studioHome, err)
		}
		line(out, "ok      deleted")
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Done. To remove the studio binary: make uninstall")
	return nil
}
