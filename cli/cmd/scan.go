package cmd

import (
	"fmt"
	"time"

	"github.com/maximilianbuff/agent-studio/internal/config"
	"github.com/maximilianbuff/agent-studio/internal/db"
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

	// Open DB — non-fatal: scan still works without it.
	d, dbErr := db.Open()
	if dbErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warn: DB unavailable: %v\n", dbErr)
	}
	if d != nil {
		defer d.Close()
	}

	fmt.Fprintf(out, "Scanning %d repo(s)...\n", enabled)
	items, err := scanner.Run(cfg, d)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Fprintln(out, "No actionable items found.")
	} else {
		fmt.Fprintf(out, "%-6s  %-8s  %-32s  %s\n", "SCORE", "TYPE", "REPO", "ITEM")
		fmt.Fprintf(out, "%-6s  %-8s  %-32s  %s\n", "-----", "----", "----", "----")
		for _, item := range items {
			title := item.Title
			if len(title) > 48 {
				title = title[:45] + "..."
			}
			issueType := item.IssueType
			if issueType == "" {
				issueType = "-"
			}
			fmt.Fprintf(out, "%-6d  %-8s  %-32s  #%d %s\n",
				item.Score, issueType, item.Repo, item.Number, title)
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
