// Package scanner scans GitHub repos and scores work items using only gh CLI calls.
// No LLM invocations — deterministic and token-free.
package scanner

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/maximilianbuff/agent-studio/internal/config"
	"github.com/maximilianbuff/agent-studio/internal/queue"
)

type ghIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Labels    []ghLabel `json:"labels"`
	Assignees []ghUser  `json:"assignees"`
	URL       string    `json:"url"`
}

type ghLabel struct{ Name string `json:"name"` }
type ghUser struct{ Login string `json:"login"` }

// Run scans all configured repos and returns scored queue items, sorted by score desc.
func Run(cfg config.Config) ([]queue.Item, error) {
	var items []queue.Item

	for _, repo := range cfg.Repos {
		if repo.Disabled {
			continue
		}
		got, err := scanRepo(cfg, repo)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", repo.Repo, err)
		}
		items = append(items, got...)
	}

	prItems, err := scanOwnPRs(cfg)
	if err == nil {
		items = append(items, prItems...)
	}

	slices.SortFunc(items, func(a, b queue.Item) int { return b.Score - a.Score })
	return items, nil
}

func scanRepo(cfg config.Config, repo config.RepoEntry) ([]queue.Item, error) {
	issues, err := listIssues(repo.Repo)
	if err != nil {
		return nil, err
	}

	// Single call to get all open PR head branches — avoids N+1 per issue.
	activeBranches, err := openPRBranches(repo.Repo)
	if err != nil {
		return nil, err
	}

	weight := repo.Priority
	if weight <= 0 {
		weight = 1.0
	}

	skipSet := make(map[string]bool, len(cfg.SkipLabels))
	for _, l := range cfg.SkipLabels {
		skipSet[l] = true
	}

	var items []queue.Item
	for _, issue := range issues {
		if shouldSkip(cfg, skipSet, issue) {
			continue
		}
		if activeBranches[fmt.Sprintf("issue/%d", issue.Number)] {
			continue
		}
		score := scoreIssue(cfg, issue, weight)
		if score < cfg.MinScore {
			continue
		}
		items = append(items, queue.Item{
			Type:       "issue",
			Repo:       repo.Repo,
			Number:     issue.Number,
			Title:      issue.Title,
			URL:        issue.URL,
			Score:      score,
			RepoWeight: weight,
		})
	}
	return items, nil
}

func shouldSkip(cfg config.Config, skipSet map[string]bool, issue ghIssue) bool {
	for _, l := range issue.Labels {
		if skipSet[l.Name] {
			return true
		}
	}
	if len(issue.Assignees) > 0 {
		for _, a := range issue.Assignees {
			if a.Login == cfg.MyLogin {
				return false
			}
		}
		return true
	}
	return false
}

func scoreIssue(cfg config.Config, issue ghIssue, weight float64) int {
	base := 0
	for _, l := range issue.Labels {
		if s, ok := cfg.Labels[l.Name]; ok {
			base += s
		}
	}
	text := strings.ToLower(issue.Title + " " + issue.Body)
	for kw, s := range cfg.Keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			base += s
		}
	}
	return int(float64(base) * weight)
}

func listIssues(repo string) ([]ghIssue, error) {
	out, err := gh("issue", "list",
		"--repo", repo,
		"--state", "open",
		"--limit", "100",
		"--json", "number,title,body,labels,assignees,url",
	)
	if err != nil {
		return nil, err
	}
	var issues []ghIssue
	return issues, json.Unmarshal(out, &issues)
}

// openPRBranches returns a set of head branch names for all open PRs in the repo.
func openPRBranches(repo string) (map[string]bool, error) {
	out, err := gh("pr", "list",
		"--repo", repo,
		"--state", "open",
		"--limit", "200",
		"--json", "headRefName",
	)
	if err != nil {
		return nil, err
	}
	var prs []struct {
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(prs))
	for _, pr := range prs {
		set[pr.HeadRefName] = true
	}
	return set, nil
}

// scanOwnPRs finds open PRs authored by the current user with changes requested.
func scanOwnPRs(cfg config.Config) ([]queue.Item, error) {
	out, err := gh("search", "prs",
		"--author", "@me",
		"--state", "open",
		"--review", "changes_requested",
		"--limit", "50",
		"--json", "number,title,url,repository",
	)
	if err != nil {
		return nil, err
	}
	var prs []struct {
		Number     int    `json:"number"`
		Title      string `json:"title"`
		URL        string `json:"url"`
		Repository struct {
			NameWithOwner string `json:"nameWithOwner"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, err
	}

	repoWeight := make(map[string]float64, len(cfg.Repos))
	for _, r := range cfg.Repos {
		if r.Priority > 0 {
			repoWeight[r.Repo] = r.Priority
		}
	}

	items := make([]queue.Item, 0, len(prs))
	for _, pr := range prs {
		repo := pr.Repository.NameWithOwner
		w := repoWeight[repo]
		if w <= 0 {
			w = 1.0
		}
		items = append(items, queue.Item{
			Type:       "pr_review",
			Repo:       repo,
			Number:     pr.Number,
			Title:      pr.Title,
			URL:        pr.URL,
			Score:      int(30 * w),
			RepoWeight: w,
		})
	}
	return items, nil
}

func gh(args ...string) ([]byte, error) {
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args[:min(2, len(args))], " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args[:min(2, len(args))], " "), err)
	}
	return out, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
