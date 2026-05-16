// Package scanner scans GitHub repos and stores work data via only gh CLI calls.
// No LLM invocations — deterministic and token-free.
// Scoring/prioritization is NOT done here; each job owns its own ranking.
package scanner

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/maximilianbuff/agent-studio/internal/config"
	"github.com/maximilianbuff/agent-studio/internal/db"
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

type ghPR struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	State       string `json:"state"`          // OPEN, CLOSED, MERGED
	MergedAt    string `json:"mergedAt"`
	HeadRefName string `json:"headRefName"`    // e.g. "issue/42"
	Mergeable   string `json:"mergeable"`      // MERGEABLE, CONFLICTING, UNKNOWN
	ReviewDecision string `json:"reviewDecision"` // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, ""
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
	StatusCheckRollup []struct {
		State string `json:"state"` // SUCCESS, FAILURE, ERROR, PENDING, ...
	} `json:"statusCheckRollup"`
}

// IssueItem is an issue found during scanning, with its computed score.
// Returned for display; actual storage is via d.UpsertItem.
type IssueItem struct {
	Repo      string
	Number    int
	Title     string
	URL       string
	IssueType string
	Score     int
}

// Run scans all configured repos:
//   - issues → upserted into DB with computed scores
//   - all open PRs per repo → upserted into DB as PRRecord
//
// Returns IssueItems for display. d may be nil — DB writes are skipped when absent.
func Run(cfg config.Config, d *db.DB) ([]IssueItem, error) {
	typeWeights := cfg.EffectiveIssueTypeWeights()
	var allIssues []IssueItem

	for _, repo := range cfg.Repos {
		if repo.Disabled {
			continue
		}

		issues, err := scanRepoIssues(cfg, typeWeights, repo, d)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", repo.Repo, err)
		}
		allIssues = append(allIssues, issues...)

		if d != nil {
			if err := scanRepoPRs(d, repo.Repo); err != nil {
				// Non-fatal: PR scan failure doesn't abort issue scan.
				fmt.Fprintf(errWriter{}, "warn: PR scan %s: %v\n", repo.Repo, err)
			}
		}
	}

	return allIssues, nil
}

// ─── Issue scanning ──────────────────────────────────────────────────────────

func scanRepoIssues(cfg config.Config, typeWeights map[string]float64, repo config.RepoEntry, d *db.DB) ([]IssueItem, error) {
	issues, err := listIssues(repo.Repo)
	if err != nil {
		return nil, err
	}

	// Single call per repo — avoids N+1 per issue.
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

	var items []IssueItem
	for _, issue := range issues {
		if shouldSkip(cfg, skipSet, issue) {
			continue
		}
		if activeBranches[fmt.Sprintf("issue/%d", issue.Number)] {
			continue
		}

		issueType := detectType(typeWeights, issue.Labels)
		typeMultiplier := typeWeights[issueType]
		if typeMultiplier <= 0 {
			typeMultiplier = 1.0
		}

		base := labelScore(cfg, issue) + keywordScore(cfg, issue)
		score := int(float64(base) * typeMultiplier * weight)
		if score < cfg.MinScore {
			continue
		}

		if d != nil {
			id, err := d.UpsertItem(repo.Repo, issue.Number, issueType, issue.Title, score)
			if err == nil {
				_ = d.AddEvent(id, "queued", "")
			}
		}

		items = append(items, IssueItem{
			Repo:      repo.Repo,
			Number:    issue.Number,
			Title:     issue.Title,
			URL:       issue.URL,
			IssueType: issueType,
			Score:     score,
		})
	}
	return items, nil
}

// detectType picks the label with the highest type weight.
func detectType(typeWeights map[string]float64, labels []ghLabel) string {
	best, bestW := "", 0.0
	for _, l := range labels {
		if w, ok := typeWeights[l.Name]; ok && w > bestW {
			best, bestW = l.Name, w
		}
	}
	return best
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

func labelScore(cfg config.Config, issue ghIssue) int {
	total := 0
	for _, l := range issue.Labels {
		total += cfg.Labels[l.Name]
	}
	return total
}

func keywordScore(cfg config.Config, issue ghIssue) int {
	text := strings.ToLower(issue.Title + " " + issue.Body)
	total := 0
	for kw, s := range cfg.Keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			total += s
		}
	}
	return total
}

// ─── PR scanning ─────────────────────────────────────────────────────────────

// scanRepoPRs fetches all open PRs for a repo and upserts them into the DB.
// Any author, any state — the prioritizer decides what's relevant per job.
func scanRepoPRs(d *db.DB, repo string) error {
	prs, err := listRepoPRs(repo)
	if err != nil {
		return err
	}
	for _, pr := range prs {
		_ = d.UpsertPR(db.PRRecord{
			Repo:           repo,
			Number:         pr.Number,
			Author:         pr.Author.Login,
			Title:          pr.Title,
			URL:            pr.URL,
			HeadRef:        pr.HeadRefName,
			State:          pr.State,
			Mergeable:      pr.Mergeable,
			ReviewDecision: pr.ReviewDecision,
			CIStatus:       aggregateCIStatus(pr),
			MergedAt:       pr.MergedAt,
		})
	}
	return nil
}

// aggregateCIStatus reduces the statusCheckRollup slice to a single string.
// Returns the worst status seen: ERROR > FAILURE > PENDING > SUCCESS > "".
func aggregateCIStatus(pr ghPR) string {
	result := ""
	for _, c := range pr.StatusCheckRollup {
		switch c.State {
		case "ERROR":
			return "ERROR"
		case "FAILURE":
			result = "FAILURE"
		case "PENDING":
			if result != "FAILURE" {
				result = "PENDING"
			}
		case "SUCCESS":
			if result == "" {
				result = "SUCCESS"
			}
		}
	}
	return result
}

// ─── gh calls ────────────────────────────────────────────────────────────────

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

func listRepoPRs(repo string) ([]ghPR, error) {
	out, err := gh("pr", "list",
		"--repo", repo,
		"--state", "open",
		"--limit", "200",
		"--json", "number,title,url,state,mergedAt,headRefName,mergeable,reviewDecision,statusCheckRollup,author",
	)
	if err != nil {
		return nil, err
	}
	var prs []ghPR
	return prs, json.Unmarshal(out, &prs)
}

func gh(args ...string) ([]byte, error) {
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			n := 2
			if len(args) < n {
				n = len(args)
			}
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args[:n], " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("gh: %w", err)
	}
	return out, nil
}

// errWriter satisfies io.Writer for non-fatal warnings.
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) {
	fmt.Print(string(p))
	return len(p), nil
}
