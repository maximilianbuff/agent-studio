package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/maximilianbuff/agent-studio/internal/config"
	"github.com/maximilianbuff/agent-studio/internal/crontab"
	"github.com/maximilianbuff/agent-studio/internal/home"
	"github.com/spf13/cobra"
)

var jobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Manage scheduled AgentStudio jobs",
}

var jobsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scheduled jobs with status and last-run time",
	RunE:  runJobsList,
}

var jobsRunCmd = &cobra.Command{
	Use:   "run <job>",
	Short: "Trigger a job immediately (foreground)",
	Args:  cobra.ExactArgs(1),
	RunE:  runJobsRun,
}

var jobsEnableCmd = &cobra.Command{
	Use:   "enable <job>",
	Short: "Re-enable a disabled job",
	Args:  cobra.ExactArgs(1),
	RunE:  runJobsEnable,
}

var jobsDisableCmd = &cobra.Command{
	Use:   "disable <job>",
	Short: "Pause a job without removing its schedule",
	Args:  cobra.ExactArgs(1),
	RunE:  runJobsDisable,
}

var jobsAddCmd = &cobra.Command{
	Use:   "add <job> <schedule>",
	Short: "Add a new cron job (job name must match a prompts/<job>.md file)",
	Args:  cobra.ExactArgs(2),
	RunE:  runJobsAdd,
}

var jobsRemoveCmd = &cobra.Command{
	Use:   "remove <job>",
	Short: "Remove a job from the cron schedule entirely",
	Args:  cobra.ExactArgs(1),
	RunE:  runJobsRemove,
}

var jobsLogsCmd = &cobra.Command{
	Use:   "logs <job>",
	Short: "Show log output for a job",
	Args:  cobra.ExactArgs(1),
	RunE:  runJobsLogs,
}

var jobsSetCmd = &cobra.Command{
	Use:   "set <job>",
	Short: "Set schedule or concurrency for a job",
	Args:  cobra.ExactArgs(1),
	RunE:  runJobsSet,
}

func init() {
	jobsListCmd.Flags().StringP("output", "o", "", "Output format: json")
	jobsLogsCmd.Flags().BoolP("tail", "f", false, "Follow log output (like tail -f)")
	jobsLogsCmd.Flags().IntP("lines", "n", 50, "Number of lines to show")
	jobsSetCmd.Flags().StringP("interval", "i", "", "Cron schedule expression (e.g. \"*/10 * * * *\")")
	jobsSetCmd.Flags().IntP("concurrency", "c", 0, "Max concurrent instances (worker only)")

	jobsCmd.AddCommand(
		jobsListCmd,
		jobsRunCmd,
		jobsEnableCmd,
		jobsDisableCmd,
		jobsAddCmd,
		jobsRemoveCmd,
		jobsLogsCmd,
		jobsSetCmd,
	)
}

type jobRow struct {
	Job         string `json:"job"`
	Schedule    string `json:"schedule"`
	Concurrency int    `json:"concurrency"`
	Enabled     bool   `json:"enabled"`
	LastRun     string `json:"last_run,omitempty"`
}

func runJobsList(cmd *cobra.Command, _ []string) error {
	outputFmt, _ := cmd.Flags().GetString("output")

	cfg, _ := config.Load()

	cronEntries, err := crontab.GetEntries(crontab.DefaultRunCmd)
	if err != nil {
		return err
	}
	cronMap := make(map[string]crontab.Entry, len(cronEntries))
	for _, e := range cronEntries {
		cronMap[e.Job] = e
	}

	// Collect known jobs: built-ins + any in crontab.
	seen := map[string]bool{"scan": true, "worker": true}
	for _, e := range cronEntries {
		seen[e.Job] = true
	}

	var rows []jobRow
	for job := range seen {
		e, inCron := cronMap[job]
		enabled := inCron && e.Enabled
		rows = append(rows, jobRow{
			Job:         job,
			Schedule:    cfg.JobInterval(job),
			Concurrency: cfg.JobConcurrency(job),
			Enabled:     enabled,
			LastRun:     lastRunTime(job),
		})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Job < rows[j].Job })

	out := cmd.OutOrStdout()

	if outputFmt == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	if len(rows) == 0 {
		fmt.Fprintln(out, "No jobs found. Run 'studio install' to set up default jobs.")
		return nil
	}

	fmt.Fprintf(out, "%-10s  %-20s  %-11s  %-8s  %s\n", "JOB", "SCHEDULE", "CONCURRENCY", "STATUS", "LAST RUN")
	fmt.Fprintf(out, "%-10s  %-20s  %-11s  %-8s  %s\n", "---", "--------", "-----------", "------", "--------")
	for _, r := range rows {
		status := "disabled"
		if r.Enabled {
			status = "enabled"
		}
		fmt.Fprintf(out, "%-10s  %-20s  %-11d  %-8s  %s\n",
			r.Job, r.Schedule, r.Concurrency, status, r.LastRun)
	}
	return nil
}

func lastRunTime(job string) string {
	logFile := home.Path("logs", job+".log")
	info, err := os.Stat(logFile)
	if err != nil {
		return "never"
	}
	return humanDuration(time.Since(info.ModTime())) + " ago"
}

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}


func runJobsRun(cmd *cobra.Command, args []string) error {
	job := args[0]

	// scan is a native command — no runner script needed.
	if job == "scan" {
		return runScan(cmd, nil)
	}

	runner := home.Path("bin", "agent-studio-run")
	if _, err := os.Stat(runner); os.IsNotExist(err) {
		return fmt.Errorf("runner not found at %s — run 'studio install' first", runner)
	}

	c := exec.Command(runner, job)
	c.Stdin = os.Stdin
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	return c.Run()
}

func runJobsSet(cmd *cobra.Command, args []string) error {
	job := args[0]
	interval, _ := cmd.Flags().GetString("interval")
	concurrency, _ := cmd.Flags().GetInt("concurrency")

	if interval == "" && concurrency == 0 {
		return fmt.Errorf("specify at least one of --interval (-i) or --concurrency (-c)")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	config.SetJob(&cfg, job, interval, concurrency)

	if err := config.Save(cfg); err != nil {
		return err
	}

	// Update live crontab: refresh schedule for this job.
	entries, err := crontab.GetEntries(crontab.DefaultRunCmd)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "warn  crontab not updated: %v\n", err)
	} else {
		for i, e := range entries {
			if e.Job == job {
				entries[i].Schedule = cfg.JobInterval(job)
			}
		}
		if err := crontab.SetEntries(entries, home.Dir(), crontab.DefaultRunCmd, crontab.DefaultRunCmdStdin); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "warn  crontab not updated: %v\n", err)
		}
	}

	commitConfig(fmt.Sprintf("config: set job %s", job))

	out := cmd.OutOrStdout()
	if interval != "" {
		fmt.Fprintf(out, "ok  %s interval = %s\n", job, interval)
	}
	if concurrency > 0 {
		fmt.Fprintf(out, "ok  %s concurrency = %d\n", job, concurrency)
	}
	return nil
}

func runJobsEnable(cmd *cobra.Command, args []string) error {
	return setJobEnabled(cmd, args[0], true)
}

func runJobsDisable(cmd *cobra.Command, args []string) error {
	return setJobEnabled(cmd, args[0], false)
}

func setJobEnabled(cmd *cobra.Command, job string, enabled bool) error {
	entries, err := crontab.GetEntries(crontab.DefaultRunCmd)
	if err != nil {
		return err
	}

	found := false
	for i, e := range entries {
		if e.Job == job {
			entries[i].Enabled = enabled
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("job %q not found in crontab", job)
	}

	if err := crontab.SetEntries(entries, home.Dir(), crontab.DefaultRunCmd, crontab.DefaultRunCmdStdin); err != nil {
		return err
	}

	state := "enabled"
	if !enabled {
		state = "disabled"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ok  %s %s\n", job, state)
	return nil
}

func runJobsAdd(cmd *cobra.Command, args []string) error {
	job, schedule := args[0], args[1]
	studioHome := home.Dir()

	promptFile := filepath.Join(studioHome, "prompts", job+".md")
	if _, err := os.Stat(promptFile); os.IsNotExist(err) {
		return fmt.Errorf("prompt file not found: %s\nCreate it first with 'studio prompts edit %s'", promptFile, job)
	}

	entries, err := crontab.GetEntries(crontab.DefaultRunCmd)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.Job == job {
			return fmt.Errorf("job %q already exists (use 'studio jobs enable' to re-enable it)", job)
		}
	}

	entries = append(entries, crontab.Entry{Job: job, Schedule: schedule, Enabled: true})
	if err := crontab.SetEntries(entries, studioHome, crontab.DefaultRunCmd, crontab.DefaultRunCmdStdin); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "ok  added job %s (%s)\n", job, schedule)
	return nil
}

func runJobsRemove(cmd *cobra.Command, args []string) error {
	job := args[0]

	entries, err := crontab.GetEntries(crontab.DefaultRunCmd)
	if err != nil {
		return err
	}

	filtered := entries[:0]
	found := false
	for _, e := range entries {
		if e.Job == job {
			found = true
		} else {
			filtered = append(filtered, e)
		}
	}
	if !found {
		return fmt.Errorf("job %q not found in crontab", job)
	}

	if err := crontab.SetEntries(filtered, home.Dir(), crontab.DefaultRunCmd, crontab.DefaultRunCmdStdin); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "ok  removed job %s\n", job)
	return nil
}

func runJobsLogs(cmd *cobra.Command, args []string) error {
	job := args[0]
	follow, _ := cmd.Flags().GetBool("tail")
	lines, _ := cmd.Flags().GetInt("lines")

	logFile := home.Path("logs", job+".log")

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Fprintf(cmd.OutOrStdout(), "No log file yet for job %q.\n", job)
		return nil
	}

	tailArgs := []string{"-n", fmt.Sprintf("%d", lines)}
	if follow {
		tailArgs = append(tailArgs, "-f")
	}
	tailArgs = append(tailArgs, logFile)

	c := exec.Command("tail", tailArgs...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()

	if follow {
		// Forward SIGINT/SIGTERM so tail exits cleanly.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sig
			if c.Process != nil {
				c.Process.Signal(syscall.SIGTERM)
			}
		}()
	}

	return c.Run()
}
