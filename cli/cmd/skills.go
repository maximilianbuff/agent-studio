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

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage Claude skills (custom named prompt files)",
}

var skillsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Scaffold a new skill definition",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsCreate,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all defined skills",
	RunE:  runSkillsList,
}

var skillsShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Print a skill definition",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsShow,
}

var skillsEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Open a skill in $EDITOR and auto-commit on save",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsEdit,
}

func init() {
	skillsCreateCmd.Flags().StringP("description", "d", "", "One-line description of what the skill does")
	skillsCmd.AddCommand(skillsCreateCmd, skillsListCmd, skillsShowCmd, skillsEditCmd)
}

func skillDir(name string) string {
	return home.Path("skills", name)
}

func skillFile(name string) string {
	return filepath.Join(skillDir(name), "skill.md")
}

const skillStub = `---
name: %s
description: %s
trigger: manual
allowed-tools:
  - Bash
  - gh
---

# %s

<!-- Describe what this skill does. The agent follows these instructions when triggered. -->

## Steps

1. <!-- Add your instructions here -->
`

func runSkillsCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	desc, _ := cmd.Flags().GetString("description")
	if desc == "" {
		desc = name
	}

	p := skillFile(name)
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("skill %q already exists: %s", name, p)
	}

	if err := os.MkdirAll(skillDir(name), 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf(skillStub, name, desc, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return err
	}

	studioHome := home.Dir()
	if gitops.IsRepo(studioHome) {
		_ = gitops.AddAll(studioHome)
		_ = gitops.Commit(studioHome, fmt.Sprintf("skills: create %s", name))
		if remote := gitops.RemoteGet(studioHome); remote != "" {
			_ = gitops.Push(studioHome)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created skill: %s\n", p)
	fmt.Fprintf(cmd.OutOrStdout(), "Edit with: studio skills edit %s\n", name)
	return nil
}

func runSkillsList(cmd *cobra.Command, _ []string) error {
	dir := home.Path("skills")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		fmt.Fprintln(cmd.OutOrStdout(), "No skills defined yet. Create one with 'studio skills create <name>'.")
		return nil
	}
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	found := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name(), "skill.md")
		if _, err := os.Stat(p); err == nil {
			desc := readSkillDescription(p)
			fmt.Fprintf(out, "%-20s  %s\n", e.Name(), desc)
			found = true
		}
	}
	if !found {
		fmt.Fprintln(out, "No skills defined yet. Create one with 'studio skills create <name>'.")
	}
	return nil
}

func runSkillsShow(cmd *cobra.Command, args []string) error {
	p := skillFile(args[0])
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return fmt.Errorf("skill %q not found: %s", args[0], p)
	}
	if err != nil {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), string(data))
	return nil
}

func runSkillsEdit(cmd *cobra.Command, args []string) error {
	name := args[0]
	p := skillFile(name)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return fmt.Errorf("skill %q not found — create it first with 'studio skills create %s'", name, name)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
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
		_ = gitops.Commit(studioHome, fmt.Sprintf("skills: update %s", name))
		if remote := gitops.RemoteGet(studioHome); remote != "" {
			_ = gitops.Push(studioHome)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "ok  saved and committed %s\n", p)
	return nil
}

// readSkillDescription extracts the description field from YAML front-matter.
func readSkillDescription(p string) string {
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "description:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return ""
}
