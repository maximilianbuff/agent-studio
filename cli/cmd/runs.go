package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/maximilianbuff/agent-studio/internal/db"
	"github.com/spf13/cobra"
)

var runsCmd = &cobra.Command{
	Use:   "runs",
	Short: "View worker run history",
}

var runsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show recent worker runs",
	RunE:  runRunsList,
}

var runsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show full detail for a single run",
	Args:  cobra.ExactArgs(1),
	RunE:  runRunsShow,
}

var runsRecordCmd = &cobra.Command{
	Use:    "record",
	Short:  "Persist a completed run (called by agent-studio-run)",
	Hidden: true,
	RunE:   runRunsRecord,
}

func init() {
	runsListCmd.Flags().IntP("limit", "n", 10, "Number of runs to show")

	runsRecordCmd.Flags().String("job", "", "Job name")
	runsRecordCmd.Flags().String("start-time", "", "ISO8601 start time")
	runsRecordCmd.Flags().String("end-time", "", "ISO8601 end time")
	runsRecordCmd.Flags().Int("duration", 0, "Duration in seconds")
	runsRecordCmd.Flags().Int("input-tokens", 0, "Input tokens consumed")
	runsRecordCmd.Flags().Int("output-tokens", 0, "Output tokens generated")
	runsRecordCmd.Flags().Int("exit-code", 0, "Claude exit code")
	runsRecordCmd.Flags().String("summary", "", "Run summary text")
	_ = runsRecordCmd.MarkFlagRequired("job")
	_ = runsRecordCmd.MarkFlagRequired("start-time")
	_ = runsRecordCmd.MarkFlagRequired("end-time")

	runsCmd.AddCommand(runsListCmd, runsShowCmd, runsRecordCmd)
}

func runRunsList(cmd *cobra.Command, _ []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	d, err := db.Open()
	if err != nil {
		return fmt.Errorf("opening DB: %w", err)
	}
	defer d.Close()

	runs, err := d.ListRuns(limit)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(runs) == 0 {
		fmt.Fprintln(out, "No runs recorded yet.")
		return nil
	}

	fmt.Fprintf(out, "%-4s  %-8s  %-20s  %-8s  %-7s  %-7s  %s\n",
		"ID", "JOB", "STARTED", "DURATION", "IN TOK", "OUT TOK", "SUMMARY")
	fmt.Fprintf(out, "%-4s  %-8s  %-20s  %-8s  %-7s  %-7s  %s\n",
		"--", "---", "-------", "--------", "------", "-------", "-------")

	for _, r := range runs {
		started := formatRunTime(r.StartedAt)
		dur := formatDurationSecs(r.DurationSecs)
		excerpt := summaryExcerpt(r.Summary, 50)
		status := ""
		if r.ExitCode != 0 {
			status = fmt.Sprintf(" [exit %d]", r.ExitCode)
		}
		fmt.Fprintf(out, "%-4d  %-8s  %-20s  %-8s  %-7d  %-7d  %s%s\n",
			r.ID, r.Job, started, dur, r.InputTokens, r.OutputTokens, excerpt, status)
	}
	return nil
}

func runRunsShow(cmd *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid id %q", args[0])
	}

	d, err := db.Open()
	if err != nil {
		return fmt.Errorf("opening DB: %w", err)
	}
	defer d.Close()

	r, err := d.GetRun(id)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Run #%d — %s\n", r.ID, r.Job)
	fmt.Fprintf(out, "Started:  %s\n", r.StartedAt)
	fmt.Fprintf(out, "Ended:    %s\n", r.EndedAt)
	fmt.Fprintf(out, "Duration: %s\n", formatDurationSecs(r.DurationSecs))
	fmt.Fprintf(out, "Tokens:   %d in / %d out\n", r.InputTokens, r.OutputTokens)
	if r.ExitCode != 0 {
		fmt.Fprintf(out, "Exit:     %d\n", r.ExitCode)
	}
	fmt.Fprintln(out)
	if r.Summary == "" {
		fmt.Fprintln(out, "(no summary)")
	} else {
		fmt.Fprintln(out, r.Summary)
	}
	return nil
}

func runRunsRecord(cmd *cobra.Command, _ []string) error {
	job, _ := cmd.Flags().GetString("job")
	startTime, _ := cmd.Flags().GetString("start-time")
	endTime, _ := cmd.Flags().GetString("end-time")
	duration, _ := cmd.Flags().GetInt("duration")
	inputTokens, _ := cmd.Flags().GetInt("input-tokens")
	outputTokens, _ := cmd.Flags().GetInt("output-tokens")
	exitCode, _ := cmd.Flags().GetInt("exit-code")
	summary, _ := cmd.Flags().GetString("summary")

	d, err := db.Open()
	if err != nil {
		return fmt.Errorf("opening DB: %w", err)
	}
	defer d.Close()

	id, err := d.InsertRun(db.RunRecord{
		Job:          job,
		StartedAt:    startTime,
		EndedAt:      endTime,
		DurationSecs: duration,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		ExitCode:     exitCode,
		Summary:      summary,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ok  run #%d recorded\n", id)
	return nil
}

func formatRunTime(s string) string {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format("2006-01-02 15:04")
		}
	}
	return s
}

func formatDurationSecs(secs int) string {
	switch {
	case secs < 60:
		return fmt.Sprintf("%ds", secs)
	case secs < 3600:
		return fmt.Sprintf("%dm%ds", secs/60, secs%60)
	default:
		h := secs / 3600
		m := (secs % 3600) / 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}
}

func summaryExcerpt(s string, max int) string {
	s = strings.TrimSpace(s)
	// Take first line
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}
