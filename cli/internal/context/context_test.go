package context_test

import (
	"os"
	"testing"

	repoctx "github.com/maximilianbuff/agent-studio/internal/context"
)

func TestReadWriteClear(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_STUDIO_HOME", dir)

	repo := "owner/repo"
	content := "# owner/repo — agent context\nLast updated: 2026-01-01T00:00Z\n"

	got, err := repoctx.Read(repo)
	if err != nil {
		t.Fatalf("Read on missing file: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	if err := repoctx.Write(repo, content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err = repoctx.Read(repo)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != content {
		t.Fatalf("expected %q, got %q", content, got)
	}

	if err := repoctx.Clear(repo); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if _, err := os.Stat(repoctx.Path(repo)); !os.IsNotExist(err) {
		t.Fatal("file should not exist after Clear")
	}

	if err := repoctx.Clear(repo); err != nil {
		t.Fatalf("Clear on missing file should be no-op: %v", err)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_STUDIO_HOME", dir)

	repos, err := repoctx.List()
	if err != nil {
		t.Fatalf("List on empty dir: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected empty list, got %v", repos)
	}

	for _, repo := range []string{"owner/repo", "another/project"} {
		if err := repoctx.Write(repo, "content"); err != nil {
			t.Fatalf("Write %s: %v", repo, err)
		}
	}

	repos, err = repoctx.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d: %v", len(repos), repos)
	}
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_STUDIO_HOME", dir)

	p := repoctx.Path("foo/bar")
	if p == "" {
		t.Fatal("Path returned empty string")
	}
	// Should contain the filename "foo-bar.md"
	if base := p[len(p)-10:]; base != "foo-bar.md" {
		t.Fatalf("unexpected path suffix: %q", p)
	}
}
