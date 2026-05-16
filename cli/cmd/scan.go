package cmd

import (
	"fmt"

	"github.com/maximilianbuff/agent-studio/internal/config"
	"github.com/maximilianbuff/agent-studio/internal/db"
	"github.com/maximilianbuff/agent-studio/internal/scanner"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan configured repos and update the work DB (no AI — pure gh calls)",
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().Bool("dry-run", false, "Print what would be stored without writing to DB")
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

	// Open DB — skip when dry-run; warn and continue otherwise.
	var d *db.DB
	if !dryRun {
		var dbErr error
		d, dbErr = db.Open()
		if dbErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warn: DB unavailable: %v\n", dbErr)
		}
		if d != nil {
			defer d.Close()
		}
	}

	fmt.Fprintf(out, "Scanning %d repo(s)...\n", enabled)
	items, err := scanner.Run(cfg, d)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Fprintln(out, "No actionable issues found.")
	} else {
		fmt.Fprintf(out, "%-6s  %-8s  %-32s  %s\n", "SCORE", "TYPE", "REPO", "ISSUE")
		fmt.Fprintf(out, "%-6s  %-8s  %-32s  %s\n", "-----", "----", "----", "-----")
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
		fmt.Fprintln(out, "\n(dry-run — no DB writes)")
		return nil
	}

	suffix := ""
	if d == nil {
		suffix = " (DB unavailable — run 'studio work prioritize' after DB is fixed)"
	}
	fmt.Fprintf(out, "\nDB updated: %d issue(s) stored%s\n", len(items), suffix)
	fmt.Fprintln(out, "Run 'studio work prioritize' to build the worker queue.")
	return nil
}
