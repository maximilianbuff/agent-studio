package cmd

import (
	"fmt"
	"strings"

	"github.com/maximilianbuff/agent-studio/internal/gitops"
	"github.com/maximilianbuff/agent-studio/internal/home"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Audit trail of all config and prompt changes",
	RunE:  runHistory,
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback <sha>",
	Short: "Restore config and prompts to a prior commit",
	Args:  cobra.ExactArgs(1),
	RunE:  runRollback,
}

var diffCmd = &cobra.Command{
	Use:   "diff [<sha>]",
	Short: "Show changes since HEAD (or since a specific commit)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDiff,
}

func init() {
	historyCmd.Flags().IntP("lines", "n", 20, "Number of commits to show")
}

func runHistory(cmd *cobra.Command, _ []string) error {
	n, _ := cmd.Flags().GetInt("lines")
	studioHome := home.Dir()

	if !gitops.IsRepo(studioHome) {
		return fmt.Errorf("no git repository at %s — run 'studio install' first", studioHome)
	}

	entries, err := gitops.Log(studioHome, n)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(entries) == 0 {
		fmt.Fprintln(out, "No history yet.")
		return nil
	}

	for _, e := range entries {
		fmt.Fprintln(out, e)
	}
	return nil
}

func runRollback(cmd *cobra.Command, args []string) error {
	sha := args[0]
	studioHome := home.Dir()

	if !gitops.IsRepo(studioHome) {
		return fmt.Errorf("no git repository at %s — run 'studio install' first", studioHome)
	}

	if err := gitops.Rollback(studioHome, sha); err != nil {
		return fmt.Errorf("rollback to %s: %w", sha, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "ok  rolled back to %s\n", sha)
	return nil
}

func runDiff(cmd *cobra.Command, args []string) error {
	studioHome := home.Dir()

	if !gitops.IsRepo(studioHome) {
		return fmt.Errorf("no git repository at %s — run 'studio install' first", studioHome)
	}

	ref := "HEAD"
	if len(args) > 0 {
		ref = args[0]
	}

	d, err := gitops.Diff(studioHome, ref)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	d = strings.TrimRight(d, "\n")
	if d == "" {
		fmt.Fprintln(out, "No changes.")
		return nil
	}
	fmt.Fprintln(out, d)
	return nil
}
