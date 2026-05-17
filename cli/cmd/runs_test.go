package cmd

import (
	"fmt"
	"strings"
	"testing"
)

func setupRunsHome(t *testing.T) string {
	t.Helper()
	studioHome := t.TempDir()
	out, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("install: %s: %v", out, err)
	}
	return studioHome
}

func recordRun(t *testing.T, studioHome, job, start, end string, duration, inTok, outTok int, summary string) {
	t.Helper()
	out, err := runStudio(t, studioHome, "runs", "record",
		"--job", job,
		"--start-time", start,
		"--end-time", end,
		"--duration", itoa(duration),
		"--input-tokens", itoa(inTok),
		"--output-tokens", itoa(outTok),
		"--summary", summary,
	)
	if err != nil {
		t.Fatalf("runs record: %s: %v", out, err)
	}
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

func TestRuns_emptyList(t *testing.T) {
	studioHome := setupRunsHome(t)

	output, err := runStudio(t, studioHome, "runs", "list")
	if err != nil {
		t.Fatalf("runs list: %s: %v", output, err)
	}
	if !strings.Contains(output, "No runs") {
		t.Errorf("expected 'No runs' message, got:\n%s", output)
	}
}

func TestRuns_recordAndList(t *testing.T) {
	studioHome := setupRunsHome(t)

	recordRun(t, studioHome, "worker",
		"2026-05-17T10:00:00Z", "2026-05-17T10:12:30Z",
		750, 45000, 3100,
		"Closed #5 by adding watchState(). All tests pass.",
	)

	output, err := runStudio(t, studioHome, "runs", "list")
	if err != nil {
		t.Fatalf("runs list: %s: %v", output, err)
	}

	for _, want := range []string{"worker", "45000", "3100", "Closed #5"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got:\n%s", want, output)
		}
	}
}

func TestRuns_show(t *testing.T) {
	studioHome := setupRunsHome(t)

	recordRun(t, studioHome, "worker",
		"2026-05-17T10:00:00Z", "2026-05-17T10:12:30Z",
		750, 45000, 3100,
		"Full summary text for the run.",
	)

	output, err := runStudio(t, studioHome, "runs", "show", "1")
	if err != nil {
		t.Fatalf("runs show: %s: %v", output, err)
	}

	for _, want := range []string{"Run #1", "worker", "45000", "3100", "Full summary text"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in show output, got:\n%s", want, output)
		}
	}
}

func TestRuns_showNotFound(t *testing.T) {
	studioHome := setupRunsHome(t)

	output, err := runStudio(t, studioHome, "runs", "show", "999")
	if err == nil {
		t.Fatalf("expected error for missing run, got:\n%s", output)
	}
	if !strings.Contains(output, "not found") {
		t.Errorf("expected 'not found' in error, got:\n%s", output)
	}
}

func TestRuns_limitFlag(t *testing.T) {
	studioHome := setupRunsHome(t)

	for i := 0; i < 5; i++ {
		recordRun(t, studioHome, "worker",
			"2026-05-17T10:00:00Z", "2026-05-17T10:01:00Z",
			60, 1000, 100, "run summary",
		)
	}

	output, err := runStudio(t, studioHome, "runs", "list", "--limit", "3")
	if err != nil {
		t.Fatalf("runs list --limit 3: %s: %v", output, err)
	}

	lines := 0
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.Contains(line, "worker") {
			lines++
		}
	}
	if lines != 3 {
		t.Errorf("expected 3 data rows with --limit 3, got %d:\n%s", lines, output)
	}
}
