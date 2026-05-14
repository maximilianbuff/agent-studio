package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/maximilianbuff/agent-studio/internal/gitops"
	"github.com/maximilianbuff/agent-studio/internal/home"
	"github.com/spf13/cobra"
)

var promptsCmd = &cobra.Command{
	Use:   "prompts",
	Short: "View and edit agent prompt files",
}

var promptsShowCmd = &cobra.Command{
	Use:   "show <job>",
	Short: "Print the prompt for a job",
	Args:  cobra.ExactArgs(1),
	RunE:  runPromptsShow,
}

var promptsEditCmd = &cobra.Command{
	Use:   "edit <job>",
	Short: "Open the prompt in $EDITOR and auto-commit on save",
	Args:  cobra.ExactArgs(1),
	RunE:  runPromptsEdit,
}

var promptsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all prompt files",
	RunE:  runPromptsList,
}

func init() {
	promptsCmd.AddCommand(promptsShowCmd, promptsEditCmd, promptsListCmd)
}

func promptPath(job string) string {
	return home.Path("prompts", job+".md")
}

func runPromptsShow(cmd *cobra.Command, args []string) error {
	p := promptPath(args[0])
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return fmt.Errorf("no prompt for %q — file not found: %s", args[0], p)
	}
	if err != nil {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), string(data))
	return nil
}

func runPromptsEdit(cmd *cobra.Command, args []string) error {
	job := args[0]
	p := promptPath(job)

	if _, err := os.Stat(p); os.IsNotExist(err) {
		// Create empty prompt so $EDITOR opens a new file.
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte("# "+job+" agent prompt\n\n"), 0o644); err != nil {
			return err
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Check common editors in order.
		for _, e := range []string{"nano", "vim", "vi"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("no editor found — set $EDITOR")
	}

	// Record mtime before opening.
	infoBefore, _ := os.Stat(p)

	c := exec.Command(editor, p)
	c.Stdin = os.Stdin
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	if err := c.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	infoAfter, _ := os.Stat(p)
	if infoBefore != nil && infoAfter != nil && infoAfter.ModTime().Equal(infoBefore.ModTime()) {
		fmt.Fprintln(cmd.OutOrStdout(), "no changes")
		return nil
	}

	studioHome := home.Dir()
	if gitops.IsRepo(studioHome) {
		_ = gitops.AddAll(studioHome)
		_ = gitops.Commit(studioHome, fmt.Sprintf("prompts: update %s", job))
		if remote := gitops.RemoteGet(studioHome); remote != "" {
			_ = gitops.Push(studioHome)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "ok  saved and committed %s\n", p)
	return nil
}

func runPromptsList(cmd *cobra.Command, _ []string) error {
	dir := home.Path("prompts")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		fmt.Fprintln(cmd.OutOrStdout(), "No prompts directory. Run 'studio install' first.")
		return nil
	}
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	found := false
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			job := strings.TrimSuffix(e.Name(), ".md")
			fmt.Fprintln(out, job)
			found = true
		}
	}
	if !found {
		fmt.Fprintln(out, "No prompts found.")
	}
	return nil
}
