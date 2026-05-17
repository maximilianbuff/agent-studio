package scanner_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/maximilianbuff/agent-studio/internal/config"
	"github.com/maximilianbuff/agent-studio/internal/db"
	"github.com/maximilianbuff/agent-studio/internal/scanner"
)

// fakeGHDir writes a stub gh binary to dir and prepends dir to PATH.
// prViewJSON is the JSON body returned by "gh pr view".
func fakeGHDir(t *testing.T, prViewJSON string) {
	t.Helper()
	bin := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
case "$1 $2" in
  "issue list") printf '[]'; exit 0 ;;
  "pr list")    printf '[]'; exit 0 ;;
  "pr view")    printf '%%s' '%s'; exit 0 ;;
esac
echo "fake gh: unhandled: $*" >&2
exit 1
`, prViewJSON)
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
}

func TestScanRepoPRs_ReconcileStale(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_STUDIO_HOME", home)

	// Fake gh: pr list returns [] (PR 42 no longer open), pr view returns MERGED.
	prViewJSON := `{"number":42,"title":"old PR","url":"https://github.com/owner/repo/pull/42","state":"MERGED","mergedAt":"2026-05-17T00:00:00Z","headRefName":"issue/42","mergeable":"","reviewDecision":"","statusCheckRollup":[],"comments":[],"author":{"login":"bot"}}`
	fakeGHDir(t, prViewJSON)

	d, err := db.Open()
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer d.Close()

	// Seed DB with a stale OPEN PR.
	if err := d.UpsertPR(db.PRRecord{Repo: "owner/repo", Number: 42, Author: "bot", State: "OPEN"}); err != nil {
		t.Fatalf("UpsertPR: %v", err)
	}

	nums, err := d.ListOpenPRNumbersByRepo("owner/repo")
	if err != nil {
		t.Fatalf("ListOpenPRNumbersByRepo before scan: %v", err)
	}
	if len(nums) != 1 || nums[0] != 42 {
		t.Fatalf("expected [42] before scan, got %v", nums)
	}

	cfg := config.Config{
		Repos: []config.RepoEntry{{Repo: "owner/repo"}},
	}
	if _, err := scanner.Run(cfg, d); err != nil {
		t.Fatalf("scanner.Run: %v", err)
	}

	// After scan, PR 42 should be reconciled to MERGED — no longer OPEN.
	nums, err = d.ListOpenPRNumbersByRepo("owner/repo")
	if err != nil {
		t.Fatalf("ListOpenPRNumbersByRepo after scan: %v", err)
	}
	if len(nums) != 0 {
		t.Fatalf("expected no open PRs after reconciliation, got %v", nums)
	}
}

func TestScanRepoPRs_NoExtraCallsWhenNoStale(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_STUDIO_HOME", home)

	// Fake gh: pr list returns PR 10 as open. pr view should NOT be called.
	bin := t.TempDir()
	script := `#!/bin/sh
case "$1 $2" in
  "issue list") printf '[]'; exit 0 ;;
  "pr list")    printf '[{"number":10,"title":"live PR","url":"https://github.com/owner/repo/pull/10","state":"OPEN","mergedAt":"","headRefName":"issue/10","mergeable":"MERGEABLE","reviewDecision":"","statusCheckRollup":[],"comments":[],"author":{"login":"bot"}}]'; exit 0 ;;
  "pr view")    echo "pr view should not be called" >&2; exit 1 ;;
esac
echo "fake gh: unhandled: $*" >&2
exit 1
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	d, err := db.Open()
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer d.Close()

	cfg := config.Config{
		Repos: []config.RepoEntry{{Repo: "owner/repo"}},
	}
	if _, err := scanner.Run(cfg, d); err != nil {
		t.Fatalf("scanner.Run: %v", err)
	}

	nums, err := d.ListOpenPRNumbersByRepo("owner/repo")
	if err != nil {
		t.Fatalf("ListOpenPRNumbersByRepo: %v", err)
	}
	if len(nums) != 1 || nums[0] != 10 {
		t.Fatalf("expected [10], got %v", nums)
	}
}
