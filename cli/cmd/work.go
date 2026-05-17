package cmd

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/maximilianbuff/agent-studio/internal/config"
	"github.com/maximilianbuff/agent-studio/internal/db"
	"github.com/maximilianbuff/agent-studio/internal/queue"
	"github.com/spf13/cobra"
)

var workCmd = &cobra.Command{
	Use:   "work",
	Short: "View, update, and prioritize work items",
}

var workPrioritizeCmd = &cobra.Command{
	Use:   "prioritize",
	Short: "Build queue.json from DB for the worker agent (pure Go — no AI)",
	RunE:  runWorkPrioritize,
}

var workListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent work items with their current status",
	RunE:  runWorkList,
}

var workStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show per-repo statistics (informational — not used for ranking)",
	RunE:  runWorkStats,
}

var workShowCmd = &cobra.Command{
	Use:   "show <owner/repo#number>",
	Short: "Show full event history for a work item",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkShow,
}

var workUpdateCmd = &cobra.Command{
	Use:   "update <owner/repo#number>",
	Short: "Record a status change for a work item (called by worker agent)",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkUpdate,
}

func init() {
	workListCmd.Flags().IntP("limit", "n", 20, "Number of items to show")
	workUpdateCmd.Flags().String("status", "", "New status: queued, in_progress, pr_opened, changes_requested, conflicting, ci_failing, merged, closed, failed")
	workUpdateCmd.Flags().String("pr-url", "", "Pull request URL (for done/merged status)")
	_ = workUpdateCmd.MarkFlagRequired("status")

	workCmd.AddCommand(workListCmd, workStatsCmd, workShowCmd, workUpdateCmd, workPrioritizeCmd)
}

// prAttentionScore returns the priority score for an open PR.
// All open PRs score at least a base value; problem states score higher.
func prAttentionScore(pr db.PRRecord, repoWeights map[string]float64) int {
	w := repoWeights[pr.Repo]
	if w <= 0 {
		w = 1.0
	}
	switch {
	case pr.Mergeable == "CONFLICTING":
		return int(40 * w)
	case pr.ReviewDecision == "CHANGES_REQUESTED":
		return int(30 * w)
	case pr.CIStatus == "FAILURE" || pr.CIStatus == "ERROR":
		return int(25 * w)
	default:
		return int(10 * w)
	}
}

func runWorkPrioritize(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	d, err := db.Open()
	if err != nil {
		return fmt.Errorf("opening DB: %w", err)
	}
	defer d.Close()

	repoWeights := make(map[string]float64, len(cfg.Repos))
	for _, r := range cfg.Repos {
		if r.Priority > 0 {
			repoWeights[r.Repo] = r.Priority
		}
	}

	// Issues not yet worked (status not pr_opened/merged/closed).
	pending, err := d.ListPendingIssues(500)
	if err != nil {
		return err
	}

	var items []queue.Item
	for _, it := range pending {
		items = append(items, queue.Item{
			Type:      "issue",
			IssueType: it.Type,
			Repo:      it.Repo,
			Number:    it.Number,
			Title:     it.Title,
			Score:     it.Score,
		})
	}

	// All open PRs for configured repos — worker decides per-PR whether to act.
	repoNames := make([]string, 0, len(cfg.Repos))
	for _, r := range cfg.Repos {
		repoNames = append(repoNames, r.Repo)
	}
	prs, err := d.ListOpenPRs(repoNames)
	if err != nil {
		return err
	}
	for _, pr := range prs {
		items = append(items, queue.Item{
			Type:   "pr_review",
			Repo:   pr.Repo,
			Number: pr.Number,
			Title:  pr.Title,
			URL:    pr.URL,
			Score:  prAttentionScore(pr, repoWeights),
		})
	}

	slices.SortFunc(items, func(a, b queue.Item) int { return b.Score - a.Score })

	q := queue.Queue{
		ScannedAt: time.Now().UTC().Format(time.RFC3339),
		Items:     items,
	}
	if err := queue.Save(q); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Queue built: %d item(s)\n", len(items))
	for _, it := range items {
		fmt.Fprintf(out, "  [%d] %s  %s #%d %s\n", it.Score, it.Type, it.Repo, it.Number, it.Title)
	}
	return nil
}

func runWorkList(cmd *cobra.Command, _ []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	d, err := db.Open()
	if err != nil {
		return fmt.Errorf("opening DB: %w", err)
	}
	defer d.Close()

	items, err := d.ListItems(limit)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(items) == 0 {
		fmt.Fprintln(out, "No work items recorded yet. Run 'studio scan' first.")
		return nil
	}

	fmt.Fprintf(out, "%-12s  %-8s  %-30s  %-6s  %-8s  %s\n",
		"STATUS", "TYPE", "REPO", "SCORE", "UPDATED", "#TITLE")
	fmt.Fprintf(out, "%-12s  %-8s  %-30s  %-6s  %-8s  %s\n",
		"------", "----", "----", "-----", "-------", "------")

	for _, it := range items {
		status := it.Status
		if status == "" {
			status = "queued"
		}
		issueType := it.Type
		if issueType == "" {
			issueType = "-"
		}
		updated := "-"
		if it.UpdatedAt != "" {
			if t, err := time.Parse("2006-01-02T15:04:05Z", it.UpdatedAt); err == nil {
				updated = humanDuration(time.Since(t)) + " ago"
			} else if t, err := time.Parse("2006-01-02 15:04:05", it.UpdatedAt); err == nil {
				updated = humanDuration(time.Since(t)) + " ago"
			}
		}
		title := fmt.Sprintf("#%d %s", it.Number, it.Title)
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		fmt.Fprintf(out, "%-12s  %-8s  %-30s  %-6d  %-8s  %s\n",
			status, issueType, it.Repo, it.Score, updated, title)
	}
	return nil
}

func runWorkStats(cmd *cobra.Command, _ []string) error {
	d, err := db.Open()
	if err != nil {
		return fmt.Errorf("opening DB: %w", err)
	}
	defer d.Close()

	stats, err := d.RepoStats()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(stats) == 0 {
		fmt.Fprintln(out, "No data yet.")
		return nil
	}

	fmt.Fprintf(out, "%-35s  %-6s  %-4s  %-6s  %-12s  %s\n",
		"REPO", "QUEUED", "DONE", "FAILED", "SUCCESS RATE", "AVG TIME-TO-PR")
	fmt.Fprintf(out, "%-35s  %-6s  %-4s  %-6s  %-12s  %s\n",
		"----", "------", "----", "------", "------------", "--------------")

	for _, s := range stats {
		rate := "-"
		if s.Queued > 0 {
			rate = fmt.Sprintf("%.0f%%", float64(s.Done)/float64(s.Queued)*100)
		}
		avg := "-"
		if s.AvgMinutes > 0 {
			avg = formatMinutes(s.AvgMinutes)
		}
		fmt.Fprintf(out, "%-35s  %-6d  %-4d  %-6d  %-12s  %s\n",
			s.Repo, s.Queued, s.Done, s.Failed, rate, avg)
	}
	return nil
}

func runWorkShow(cmd *cobra.Command, args []string) error {
	repo, number, err := parseWorkRef(args[0])
	if err != nil {
		return err
	}

	d, err := db.Open()
	if err != nil {
		return fmt.Errorf("opening DB: %w", err)
	}
	defer d.Close()

	item, events, err := d.GetItemHistory(repo, number)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s #%d — %s\n", item.Repo, item.Number, item.Title)
	fmt.Fprintf(out, "Type: %s  Score: %d  First seen: %s\n\n",
		orDash(item.Type), item.Score, item.FirstSeen)

	if len(events) == 0 {
		fmt.Fprintln(out, "No events recorded.")
		return nil
	}

	fmt.Fprintf(out, "%-10s  %-22s  %s\n", "STATUS", "AT", "PR")
	fmt.Fprintf(out, "%-10s  %-22s  %s\n", "------", "--", "--")
	for _, e := range events {
		fmt.Fprintf(out, "%-10s  %-22s  %s\n", e.Status, e.At, orDash(e.PRUrl))
	}
	return nil
}

func runWorkUpdate(cmd *cobra.Command, args []string) error {
	repo, number, err := parseWorkRef(args[0])
	if err != nil {
		return err
	}
	status, _ := cmd.Flags().GetString("status")
	prURL, _ := cmd.Flags().GetString("pr-url")

	validStatuses := map[string]bool{
		"queued": true, "in_progress": true, "pr_opened": true,
		"changes_requested": true, "conflicting": true, "ci_failing": true,
		"merged": true, "closed": true, "failed": true,
	}
	if !validStatuses[status] {
		return fmt.Errorf("invalid status %q — valid: queued, in_progress, pr_opened, changes_requested, conflicting, ci_failing, merged, closed, failed", status)
	}

	d, err := db.Open()
	if err != nil {
		return fmt.Errorf("opening DB: %w", err)
	}
	defer d.Close()

	if err := d.AddEventByRef(repo, number, status, prURL); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ok  %s#%d → %s\n", repo, number, status)
	return nil
}

// parseWorkRef parses "owner/repo#42" into repo and number.
func parseWorkRef(ref string) (string, int, error) {
	parts := strings.SplitN(ref, "#", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid ref %q — use owner/repo#number", ref)
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid issue number in %q", ref)
	}
	return parts[0], n, nil
}

func formatMinutes(m float64) string {
	if m < 60 {
		return fmt.Sprintf("%.0fm", m)
	}
	h := int(m / 60)
	min := int(m) % 60
	if min == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, min)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
