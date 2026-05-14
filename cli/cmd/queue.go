package cmd

import (
	"fmt"

	"github.com/maximilianbuff/agent-studio/internal/queue"
	"github.com/spf13/cobra"
)

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Inspect and manage the work queue",
}

var queueShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the current work queue (sorted by score)",
	RunE:  runQueueShow,
}

var queueClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Empty the work queue",
	RunE:  runQueueClear,
}

func init() {
	queueCmd.AddCommand(queueShowCmd, queueClearCmd)
}

func runQueueShow(cmd *cobra.Command, _ []string) error {
	items, err := queue.Load()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(items) == 0 {
		fmt.Fprintln(out, "Queue is empty.")
		return nil
	}

	fmt.Fprintf(out, "%-5s  %-35s  %-7s  %s\n", "SCORE", "REPO", "ISSUE", "TITLE")
	fmt.Fprintf(out, "%-5s  %-35s  %-7s  %s\n", "-----", "----", "-----", "-----")
	for _, it := range items {
		claimed := ""
		if it.Claimed {
			claimed = " [claimed]"
		}
		fmt.Fprintf(out, "%-5d  %-35s  #%-6d %s%s\n", it.Score, it.Repo, it.Number, it.Title, claimed)
	}
	return nil
}

func runQueueClear(cmd *cobra.Command, _ []string) error {
	if err := queue.Clear(); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "ok  queue cleared")
	return nil
}
