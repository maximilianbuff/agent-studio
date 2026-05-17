package cmd

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/maximilianbuff/agent-studio/internal/assets"
	"github.com/maximilianbuff/agent-studio/internal/claudemd"
	"github.com/maximilianbuff/agent-studio/internal/config"
	"github.com/maximilianbuff/agent-studio/internal/crontab"
	"github.com/maximilianbuff/agent-studio/internal/gitops"
	"github.com/maximilianbuff/agent-studio/internal/home"
	"github.com/maximilianbuff/agent-studio/internal/prereqs"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install or update the ~/.agent-studio/ state directory and cron jobs",
	Long: `studio install sets up (or updates) AgentStudio on this machine:

  1. Checks that git, gh, and claude are on PATH
  2. Creates ~/.agent-studio/ with subdirectories
  3. Initialises a git repository inside it
  4. Copies default config, prompts, and runner script
  5. Commits everything to git
  6. Registers the scan and worker cron jobs

Safe to re-run — existing files are skipped by default.
Use --force to overwrite them (useful when updating default prompts).
Use --dry-run to preview changes without applying them.

To get the studio binary itself, see install.sh or the README.`,
	RunE: runInstall,
}

func init() {
	installCmd.Flags().Bool("dry-run", false, "Print what would be done without making changes")
	installCmd.Flags().Bool("force", false, "Overwrite existing files (use when updating)")
	installCmd.Flags().Bool("no-cron", false, "Skip crontab registration")
	installCmd.Flags().Bool("yes", false, "Accept all prompts non-interactively")
}

func runInstall(cmd *cobra.Command, _ []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	noCron, _ := cmd.Flags().GetBool("no-cron")
	yes, _ := cmd.Flags().GetBool("yes")

	out := cmd.OutOrStdout()
	studioHome := home.Dir()

	step(out, "Checking prerequisites")
	results := prereqs.CheckRequired()
	for _, r := range results {
		if r.Found {
			line(out, "ok      %s (%s)", r.Dep.Name, r.Path)
		} else {
			line(out, "MISSING %s — install from %s", r.Dep.Name, r.Dep.Hint)
		}
	}
	if err := prereqs.Err(results); err != nil {
		return err
	}

	step(out, "Creating %s", studioHome)
	dirs := []string{
		filepath.Join(studioHome, "bin"),
		filepath.Join(studioHome, "prompts"),
		filepath.Join(studioHome, "logs"),
		filepath.Join(studioHome, "context"),
	}
	for _, d := range dirs {
		line(out, "mkdir   %s", d)
		if !dryRun {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return err
			}
		}
	}

	step(out, "Copying default files")
	if err := copyDefaults(out, studioHome, dryRun, force); err != nil {
		return err
	}

	step(out, "Initialising git repository")
	if dryRun {
		line(out, "skip    (dry-run)")
	} else {
		if !gitops.IsRepo(studioHome) {
			if err := gitops.Init(studioHome); err != nil {
				return err
			}
			if err := gitops.ConfigUser(studioHome); err != nil {
				return err
			}
			line(out, "ok      git init")
		} else {
			line(out, "skip    already a git repo")
		}
		if err := gitops.AddAll(studioHome); err != nil {
			return err
		}
		if err := gitops.Commit(studioHome, "chore: init agent-studio"); err != nil {
			return err
		}
		line(out, "ok      committed")
	}

	if !noCron {
		step(out, "Registering cron jobs")
		cfg, _ := config.Load()
		defaultJobs := []crontab.Entry{
			{Job: "scan", Schedule: cfg.JobInterval("scan"), Enabled: true},
			{Job: "worker", Schedule: cfg.JobInterval("worker"), Enabled: true},
		}
		for _, e := range defaultJobs {
			line(out, "enable  %s (%s)", e.Job, e.Schedule)
		}
		if !dryRun {
			if err := crontab.SetEntries(
				defaultJobs,
				studioHome,
				crontab.DefaultRunCmd,
				crontab.DefaultRunCmdStdin,
			); err != nil {
				return fmt.Errorf("registering cron jobs: %w", err)
			}
		}
	}

	step(out, "Claude Code integration")
	if confirm(cmd, yes, dryRun,
		fmt.Sprintf("Inject AgentStudio context into %s so every Claude Code\n  session on this machine knows studio is available? [Y/n] ", claudemd.Path()),
	) {
		line(out, "inject  %s", claudemd.Path())
		if !dryRun {
			if err := claudemd.Inject(studioHome); err != nil {
				line(out, "warn    could not write %s: %v", claudemd.Path(), err)
			}
		}
	} else {
		line(out, "skip    Claude Code integration")
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Done. Run 'studio jobs list' to verify.")
	return nil
}

// copyDefaults copies embedded default files into studioHome.
// Existing files are skipped unless force is true.
func copyDefaults(out io.Writer, studioHome string, dryRun, force bool) error {
	return fs.WalkDir(assets.FS, "files", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel("files", path)
		if err != nil {
			return err
		}
		dst := filepath.Join(studioHome, rel)

		// bin/ and prompts/ are version-managed — always overwrite so updates ship on install.
		isVersionManaged := strings.HasPrefix(rel, "bin"+string(filepath.Separator)) ||
			strings.HasPrefix(rel, "prompts"+string(filepath.Separator))
		if !force && !isVersionManaged {
			if _, err := os.Stat(dst); err == nil {
				line(out, "skip    %s (already exists)", dst)
				return nil
			}
		}

		line(out, "copy    %s", dst)
		if dryRun {
			return nil
		}

		data, err := assets.FS.ReadFile(path)
		if err != nil {
			return err
		}

		mode := fs.FileMode(0o644)
		if filepath.Base(dst) == "agent-studio-run" {
			mode = 0o755
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, mode)
	})
}

// confirm asks the user a yes/no question, returning true for yes.
// Returns true immediately when yes==true or dryRun==true (dry-run previews the action).
// Defaults to yes on empty input. Returns false when stdin is not a terminal.
func confirm(cmd *cobra.Command, yes, dryRun bool, prompt string) bool {
	if dryRun || yes {
		return true
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n  %s", prompt)

	in := cmd.InOrStdin()
	// Fall back to no when stdin is not interactive (piped install.sh | sh).
	fi, ok := in.(*os.File)
	if !ok || !isTerminal(fi) {
		fmt.Fprintln(out, "y (non-interactive, defaulting to yes)")
		return true
	}

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "" || answer == "y" || answer == "yes"
}

// isTerminal reports whether f is a character device (TTY).
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func step(out io.Writer, format string, args ...any) {
	fmt.Fprintf(out, "\n%s\n", fmt.Sprintf(format, args...))
}

func line(out io.Writer, format string, args ...any) {
	fmt.Fprintf(out, "  %s\n", fmt.Sprintf(format, args...))
}
