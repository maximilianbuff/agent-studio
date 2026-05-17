package cmd

import (
	"fmt"
	"os"

	repoctx "github.com/maximilianbuff/agent-studio/internal/context"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage per-repo persistent context files",
}

var contextShowCmd = &cobra.Command{
	Use:   "show <owner/repo>",
	Short: "Print the context file for a repo",
	Args:  cobra.ExactArgs(1),
	RunE:  runContextShow,
}

var contextClearCmd = &cobra.Command{
	Use:   "clear <owner/repo>",
	Short: "Delete the context file for a repo",
	Args:  cobra.ExactArgs(1),
	RunE:  runContextClear,
}

var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all repos with a saved context",
	RunE:  runContextList,
}

func init() {
	contextCmd.AddCommand(contextShowCmd, contextClearCmd, contextListCmd)
}

func runContextShow(cmd *cobra.Command, args []string) error {
	repo := args[0]
	content, err := repoctx.Read(repo)
	if err != nil {
		return err
	}
	if content == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "no context saved for %s\n", repo)
		return nil
	}
	fmt.Fprint(cmd.OutOrStdout(), content)
	return nil
}

func runContextClear(cmd *cobra.Command, args []string) error {
	repo := args[0]
	p := repoctx.Path(repo)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		fmt.Fprintf(cmd.OutOrStdout(), "no context saved for %s\n", repo)
		return nil
	}
	if err := repoctx.Clear(repo); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ok  cleared context for %s\n", repo)
	return nil
}

func runContextList(cmd *cobra.Command, _ []string) error {
	repos, err := repoctx.List()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(repos) == 0 {
		fmt.Fprintln(out, "No context files saved.")
		return nil
	}
	for _, r := range repos {
		fmt.Fprintln(out, r)
	}
	return nil
}
