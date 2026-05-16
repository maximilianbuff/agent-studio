package cmd

import (
	"fmt"
	"time"

	"github.com/maximilianbuff/agent-studio/internal/config"
	"github.com/maximilianbuff/agent-studio/internal/queue"
	"github.com/maximilianbuff/agent-studio/internal/scanner"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan configured repos and update the work queue (no AI — pure gh calls)",
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().Bool("dry-run", false, "Print what would be queued without writing queue.json")
}

func runScan(cmd *cobra.Command, _ []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	out := cmd.OutOrStdout()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	enabled := 0
	for _, r := range cfg.Repos {
		if !r.Disabled {
			enabled++
		}
	}
	if enabled == 0 {
		fmt.Fprintln(out, "No repos configured. Add one with 'studio config repos add owner/repo'")
		return nil
	}

	fmt.Fprintf(out, "Scanning %d repo(s)...\n", enabled)
	items, err := scanner.Run(cfg)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Fprintln(out, "No actionable items found.")
	} else {
		fmt.Fprintf(out, "%-6s  %-32s  %s\n", "SCORE", "REPO", "ITEM")
		fmt.Fprintf(out, "%-6s  %-32s  %s\n", "-----", "----", "----")
		for _, item := range items {
			title := item.Title
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			fmt.Fprintf(out, "%-6d  %-32s  #%d %s\n", item.Score, item.Repo, item.Number, title)
		}
	}

	if dryRun {
		return nil
	}

	q := queue.Queue{
		ScannedAt: time.Now().UTC().Format(time.RFC3339),
		Items:     items,
	}
	if err := queue.Save(q); err != nil {
		return err
	}
	fmt.Fprintf(out, "\nQueue updated: %d item(s), scanned at %s\n", len(items), q.ScannedAt)
	return nil
}
