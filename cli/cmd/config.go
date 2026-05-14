package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/maximilianbuff/agent-studio/internal/config"
	"github.com/maximilianbuff/agent-studio/internal/gitops"
	"github.com/maximilianbuff/agent-studio/internal/home"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and edit studio configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the full configuration as JSON",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value (my_login, min_score, labels.<l>, keywords.<w>)",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

var configReposCmd = &cobra.Command{
	Use:   "repos",
	Short: "Manage the list of repositories to scan",
}

var configReposAddCmd = &cobra.Command{
	Use:   "add <owner/repo>",
	Short: "Add a repository to the scan list",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigReposAdd,
}

var configReposRemoveCmd = &cobra.Command{
	Use:   "remove <owner/repo>",
	Short: "Remove a repository from the scan list",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigReposRemove,
}

var configReposListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured repositories",
	RunE:  runConfigReposList,
}

func init() {
	configReposAddCmd.Flags().Float64P("priority", "p", 1.0, "Priority multiplier (higher = more likely to be worked)")

	configReposCmd.AddCommand(configReposAddCmd, configReposRemoveCmd, configReposListCmd)
	configCmd.AddCommand(configShowCmd, configSetCmd, configReposCmd)
}

func runConfigShow(cmd *cobra.Command, _ []string) error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	if err := config.Set(&c, args[0], args[1]); err != nil {
		return err
	}
	if err := config.Save(c); err != nil {
		return err
	}
	commitConfig(fmt.Sprintf("config: set %s", args[0]))
	fmt.Fprintf(cmd.OutOrStdout(), "ok  %s = %s\n", args[0], args[1])
	return nil
}

func runConfigReposAdd(cmd *cobra.Command, args []string) error {
	priority, _ := cmd.Flags().GetFloat64("priority")
	c, err := config.Load()
	if err != nil {
		return err
	}
	if err := config.RepoAdd(&c, args[0], priority); err != nil {
		return err
	}
	if err := config.Save(c); err != nil {
		return err
	}
	commitConfig(fmt.Sprintf("config: add repo %s", args[0]))
	fmt.Fprintf(cmd.OutOrStdout(), "ok  added %s (priority %.1f)\n", args[0], priority)
	return nil
}

func runConfigReposRemove(cmd *cobra.Command, args []string) error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	if err := config.RepoRemove(&c, args[0]); err != nil {
		return err
	}
	if err := config.Save(c); err != nil {
		return err
	}
	commitConfig(fmt.Sprintf("config: remove repo %s", args[0]))
	fmt.Fprintf(cmd.OutOrStdout(), "ok  removed %s\n", args[0])
	return nil
}

func runConfigReposList(cmd *cobra.Command, _ []string) error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(c.Repos) == 0 {
		fmt.Fprintln(out, "No repositories configured. Add one with 'studio config repos add owner/repo'")
		return nil
	}
	fmt.Fprintf(out, "%-40s  %s\n", "REPO", "PRIORITY")
	fmt.Fprintf(out, "%-40s  %s\n", "----", "--------")
	for _, r := range c.Repos {
		fmt.Fprintf(out, "%-40s  %.1f\n", r.Repo, r.Priority)
	}
	return nil
}

// commitConfig auto-commits config.json with the given message (best-effort).
func commitConfig(msg string) {
	studioHome := home.Dir()
	if !gitops.IsRepo(studioHome) {
		return
	}
	_ = gitops.AddAll(studioHome)
	_ = gitops.Commit(studioHome, msg)

	// Mirror to remote if one is configured.
	if remote := gitops.RemoteGet(studioHome); remote != "" {
		_ = gitops.Push(studioHome)
	}
}
