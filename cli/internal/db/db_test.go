package db_test

import (
	"sort"
	"testing"

	"github.com/maximilianbuff/agent-studio/internal/db"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AGENT_STUDIO_HOME", dir)
	d, err := db.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestListOpenPRNumbersByRepo(t *testing.T) {
	d := openTestDB(t)

	repo := "owner/repo"

	// Empty — no PRs yet.
	nums, err := d.ListOpenPRNumbersByRepo(repo)
	if err != nil {
		t.Fatalf("ListOpenPRNumbersByRepo on empty db: %v", err)
	}
	if len(nums) != 0 {
		t.Fatalf("expected empty, got %v", nums)
	}

	// Insert 3 PRs: 2 open, 1 merged.
	for _, pr := range []db.PRRecord{
		{Repo: repo, Number: 1, Author: "bot", State: "OPEN"},
		{Repo: repo, Number: 2, Author: "bot", State: "OPEN"},
		{Repo: repo, Number: 3, Author: "bot", State: "MERGED", MergedAt: "2026-01-01T00:00:00Z"},
	} {
		if err := d.UpsertPR(pr); err != nil {
			t.Fatalf("UpsertPR #%d: %v", pr.Number, err)
		}
	}

	nums, err = d.ListOpenPRNumbersByRepo(repo)
	if err != nil {
		t.Fatalf("ListOpenPRNumbersByRepo: %v", err)
	}
	sort.Ints(nums)
	if len(nums) != 2 || nums[0] != 1 || nums[1] != 2 {
		t.Fatalf("expected [1 2], got %v", nums)
	}

	// PRs in another repo are not included.
	other := "other/repo"
	if err := d.UpsertPR(db.PRRecord{Repo: other, Number: 10, Author: "bot", State: "OPEN"}); err != nil {
		t.Fatalf("UpsertPR other repo: %v", err)
	}
	nums, err = d.ListOpenPRNumbersByRepo(repo)
	if err != nil {
		t.Fatalf("ListOpenPRNumbersByRepo after other repo insert: %v", err)
	}
	if len(nums) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(nums), nums)
	}

	// Upserting PR 1 with MERGED drops it from the open set.
	if err := d.UpsertPR(db.PRRecord{Repo: repo, Number: 1, Author: "bot", State: "MERGED", MergedAt: "2026-05-17T00:00:00Z"}); err != nil {
		t.Fatalf("UpsertPR update to MERGED: %v", err)
	}
	nums, err = d.ListOpenPRNumbersByRepo(repo)
	if err != nil {
		t.Fatalf("ListOpenPRNumbersByRepo after merge: %v", err)
	}
	if len(nums) != 1 || nums[0] != 2 {
		t.Fatalf("expected [2], got %v", nums)
	}
}
