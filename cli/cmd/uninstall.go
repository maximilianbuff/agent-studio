package cmd

import (
	"fmt"
	"os"

	"github.com/maximilianbuff/agent-studio/internal/claudemd"
	"github.com/maximilianbuff/agent-studio/internal/crontab"
	"github.com/maximilianbuff/agent-studio/internal/home"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove AgentStudio cron jobs, Claude Code injection, and the studio binary",
	Long: `studio uninstall removes AgentStudio from this machine:

  1. Removes the AgentStudio cron block
  2. Removes the Claude Code injection in ~/.claude/CLAUDE.md
  3. Removes the studio binary (this file)
  4. With --purge: also deletes ~/.agent-studio/ and all its contents

Prompts for confirmation before any destructive action.
Use --yes to skip the first prompt; --purge --yes for a fully non-interactive wipe.`,
	RunE: runUninstall,
}

func init() {
	uninstallCmd.Flags().Bool("purge", false, "Also delete ~/.agent-studio/ and all its contents")
	uninstallCmd.Flags().Bool("yes", false, "Skip the first confirmation prompt (for scripts)")
}

func runUninstall(cmd *cobra.Command, _ []string) error {
	purge, _ := cmd.Flags().GetBool("purge")
	yes, _ := cmd.Flags().GetBool("yes")
	out := cmd.OutOrStdout()
	studioHome := home.Dir()

	self, _ := os.Executable()

	// Print summary of what will be removed.
	fmt.Fprintln(out, "Will remove:")
	fmt.Fprintln(out, "  • AgentStudio cron block")
	fmt.Fprintf(out, "  • Claude Code injection in %s\n", claudemd.Path())
	if self != "" {
		fmt.Fprintf(out, "  • Binary: %s\n", self)
	}
	fmt.Fprintln(out)

	if !yes && !confirmNo(cmd, "Continue? [y/N] ") {
		return fmt.Errorf("aborted")
	}

	// Remove cron jobs — warn on failure, continue.
	step(out, "Removing cron jobs")
	if err := crontab.DeleteBlock(crontab.DefaultRunCmd, crontab.DefaultRunCmdStdin); err != nil {
		line(out, "warn    %v", err)
	} else {
		line(out, "ok      crontab block removed")
	}

	// Remove Claude Code registration.
	step(out, "Removing Claude Code registration")
	if err := claudemd.Remove(); err != nil {
		line(out, "warn    %v", err)
	} else {
		line(out, "ok      removed from %s", claudemd.Path())
	}

	// Remove binary.
	if self != "" {
		step(out, "Removing binary")
		if err := os.Remove(self); err != nil {
			line(out, "warn    %v", err)
		} else {
			line(out, "ok      removed %s", self)
		}
	}

	// Optionally purge data directory.
	deleteData := purge
	if !deleteData {
		fmt.Fprintln(out)
		deleteData = confirmNo(cmd, fmt.Sprintf("Also delete %s and all its contents? [y/N] ", studioHome))
	}
	if deleteData {
		step(out, "Deleting %s", studioHome)
		if err := os.RemoveAll(studioHome); err != nil {
			line(out, "warn    %v", err)
		} else {
			line(out, "ok      deleted")
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Done.")
	return nil
}

// confirmNo prompts the user and returns true only for explicit "y" or "yes".
// Defaults to false (No) on empty input. Always returns false when stdin is not a TTY.
func confirmNo(cmd *cobra.Command, prompt string) bool {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "  %s", prompt)

	in := cmd.InOrStdin()
	fi, ok := in.(*os.File)
	if !ok || !isTerminal(fi) {
		fmt.Fprintln(out, "n (non-interactive, defaulting to no)")
		return false
	}

	// Read one line.
	buf := make([]byte, 256)
	n, _ := fi.Read(buf)
	answer := string(buf[:n])
	// Trim CR/LF.
	for len(answer) > 0 && (answer[len(answer)-1] == '\n' || answer[len(answer)-1] == '\r') {
		answer = answer[:len(answer)-1]
	}
	switch answer {
	case "y", "Y", "yes", "YES", "Yes":
		return true
	default:
		return false
	}
}
