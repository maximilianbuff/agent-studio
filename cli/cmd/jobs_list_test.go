package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupJobsListHome(t *testing.T) string {
	t.Helper()
	studioHome := t.TempDir()
	out, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("install: %s: %v", out, err)
	}
	return studioHome
}

func TestJobsList_showsBuiltinJobs(t *testing.T) {
	studioHome := setupJobsListHome(t)

	output, err := runStudio(t, studioHome, "jobs", "list")
	if err != nil {
		t.Fatalf("jobs list: %s: %v", output, err)
	}

	for _, job := range []string{"scan", "worker"} {
		if !strings.Contains(output, job) {
			t.Errorf("expected %q in output, got:\n%s", job, output)
		}
	}
}

func TestJobsList_showsScheduledStatus(t *testing.T) {
	studioHome := setupJobsListHome(t)

	crontabFile := filepath.Join(studioHome, "fake-crontab.txt")
	block := `# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=scan
7 */2 * * *  ` + studioHome + `/bin/agent-studio-run scan >> ` + studioHome + `/logs/scan.log 2>&1
# AGENT_STUDIO_JOB=worker
*/2 * * * *  ` + studioHome + `/bin/agent-studio-run worker >> ` + studioHome + `/logs/worker.log 2>&1
# END_AGENT_STUDIO
`
	os.WriteFile(crontabFile, []byte(block), 0o644)

	output, err := runStudio(t, studioHome, "jobs", "list")
	if err != nil {
		t.Fatalf("jobs list: %s: %v", output, err)
	}

	if !strings.Contains(output, "yes") {
		t.Errorf("expected 'yes' for scheduled jobs, got:\n%s", output)
	}
	if !strings.Contains(output, "7 */2 * * *") {
		t.Errorf("expected schedule expression in output, got:\n%s", output)
	}
}

func TestJobsList_showsUnscheduledJobs(t *testing.T) {
	studioHome := setupJobsListHome(t)

	promptPath := filepath.Join(studioHome, "prompts", "triage-issues.md")
	os.WriteFile(promptPath, []byte("Triage and label incoming issues.\n\n## Details\n..."), 0o644)

	output, err := runStudio(t, studioHome, "jobs", "list")
	if err != nil {
		t.Fatalf("jobs list: %s: %v", output, err)
	}

	if !strings.Contains(output, "triage-issues") {
		t.Errorf("expected triage-issues in output, got:\n%s", output)
	}
	if !strings.Contains(output, "no") {
		t.Errorf("expected 'no' for unscheduled job, got:\n%s", output)
	}
}

func TestJobsList_showsMissingPromptWarning(t *testing.T) {
	studioHome := setupJobsListHome(t)

	crontabFile := filepath.Join(studioHome, "fake-crontab.txt")
	block := `# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=ghost-job
*/5 * * * *  ` + studioHome + `/bin/agent-studio-run ghost-job >> ` + studioHome + `/logs/ghost-job.log 2>&1
# END_AGENT_STUDIO
`
	os.WriteFile(crontabFile, []byte(block), 0o644)

	output, err := runStudio(t, studioHome, "jobs", "list")
	if err != nil {
		t.Fatalf("jobs list: %s: %v", output, err)
	}

	if !strings.Contains(output, "ghost-job") {
		t.Errorf("expected ghost-job in output, got:\n%s", output)
	}
	if !strings.Contains(output, "MISSING") {
		t.Errorf("expected MISSING warning for job without prompt, got:\n%s", output)
	}
}

func TestJobsList_jsonOutput(t *testing.T) {
	studioHome := setupJobsListHome(t)

	crontabFile := filepath.Join(studioHome, "fake-crontab.txt")
	block := `# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=scan
7 */2 * * *  ` + studioHome + `/bin/agent-studio-run scan >> ` + studioHome + `/logs/scan.log 2>&1
# END_AGENT_STUDIO
`
	os.WriteFile(crontabFile, []byte(block), 0o644)

	output, err := runStudio(t, studioHome, "jobs", "list", "--output", "json")
	if err != nil {
		t.Fatalf("jobs list --output json: %s: %v", output, err)
	}

	var rows []jobRow
	if err := json.Unmarshal([]byte(output), &rows); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, output)
	}

	found := false
	for _, r := range rows {
		if r.Job == "scan" {
			found = true
			if !r.Scheduled {
				t.Error("scan should be scheduled=true")
			}
			if r.Schedule != "7 */2 * * *" {
				t.Errorf("scan schedule: got %q, want %q", r.Schedule, "7 */2 * * *")
			}
			if r.Description == "" {
				t.Error("scan description should not be empty")
			}
		}
	}
	if !found {
		t.Errorf("scan not found in JSON output: %s", output)
	}
}

func TestJobsList_extractDescription(t *testing.T) {
	tmp := t.TempDir()

	cases := []struct {
		content string
		want    string
	}{
		{"# Heading\n\nFirst real line.\n", "First real line."},
		{"First line no heading.\n", "First line no heading."},
		{"\n\n# H1\n\nAfter blank.\n", "After blank."},
		{"# H1\n## H2\n\nContent here.\n", "Content here."},
		{"", ""},
	}

	for _, tc := range cases {
		f := filepath.Join(tmp, "test.md")
		os.WriteFile(f, []byte(tc.content), 0o644)
		got := extractDescription(f)
		if got != tc.want {
			t.Errorf("extractDescription(%q): got %q, want %q", tc.content, got, tc.want)
		}
	}
}
