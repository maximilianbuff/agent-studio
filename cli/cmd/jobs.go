package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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

func init() {
	jobsLogsCmd.Flags().BoolP("tail", "f", false, "Follow log output (like tail -f)")
	jobsLogsCmd.Flags().IntP("lines", "n", 50, "Number of lines to show")

	jobsCmd.AddCommand(
		jobsListCmd,
		jobsRunCmd,
		jobsEnableCmd,
		jobsDisableCmd,
		jobsAddCmd,
		jobsRemoveCmd,
		jobsLogsCmd,
	)
}

func runJobsList(cmd *cobra.Command, _ []string) error {
	entries, err := crontab.GetEntries(crontab.DefaultRunCmd)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(entries) == 0 {
		fmt.Fprintln(out, "No AgentStudio jobs scheduled. Run 'studio install' to set up.")
		return nil
	}

	fmt.Fprintf(out, "%-12s  %-20s  %-8s  %s\n", "JOB", "SCHEDULE", "STATUS", "LAST RUN")
	fmt.Fprintf(out, "%-12s  %-20s  %-8s  %s\n", "---", "--------", "------", "--------")

	for _, e := range entries {
		status := "enabled"
		if !e.Enabled {
			status = "disabled"
		}
		lastRun := lastRunTime(e.Job)
		fmt.Fprintf(out, "%-12s  %-20s  %-8s  %s\n", e.Job, e.Schedule, status, lastRun)
	}
	return nil
}

func runJobsRun(cmd *cobra.Command, args []string) error {
	job := args[0]
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

// lastRunTime returns a human-readable string for the last modification time
// of the job's log file, or "never" if the log does not exist.
func lastRunTime(job string) string {
	logFile := home.Path("logs", job+".log")
	info, err := os.Stat(logFile)
	if err != nil {
		return "never"
	}
	return humanDuration(time.Since(info.ModTime()))
}

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
