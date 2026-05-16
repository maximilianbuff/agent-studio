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

func TestJobsList_showsSchedule(t *testing.T) {
	studioHome := setupJobsListHome(t)

	output, err := runStudio(t, studioHome, "jobs", "list")
	if err != nil {
		t.Fatalf("jobs list: %s: %v", output, err)
	}

	// Default intervals should appear in the output.
	for _, sched := range []string{"*/5 * * * *", "*/10 * * * *"} {
		if !strings.Contains(output, sched) {
			t.Errorf("expected schedule %q in output, got:\n%s", sched, output)
		}
	}
}

func TestJobsList_showsEnabledStatus(t *testing.T) {
	studioHome := setupJobsListHome(t)

	crontabFile := filepath.Join(studioHome, "fake-crontab.txt")
	block := `# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=scan
*/5 * * * *  ` + studioHome + `/bin/agent-studio-run scan >> ` + studioHome + `/logs/scan.log 2>&1
# AGENT_STUDIO_JOB=worker
*/10 * * * *  ` + studioHome + `/bin/agent-studio-run worker >> ` + studioHome + `/logs/worker.log 2>&1
# END_AGENT_STUDIO
`
	os.WriteFile(crontabFile, []byte(block), 0o644)

	output, err := runStudio(t, studioHome, "jobs", "list")
	if err != nil {
		t.Fatalf("jobs list: %s: %v", output, err)
	}

	if !strings.Contains(output, "enabled") {
		t.Errorf("expected 'enabled' in output, got:\n%s", output)
	}
}

func TestJobsList_showsExtraJobsFromCrontab(t *testing.T) {
	studioHome := setupJobsListHome(t)

	crontabFile := filepath.Join(studioHome, "fake-crontab.txt")
	block := `# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=triage
*/30 * * * *  ` + studioHome + `/bin/agent-studio-run triage >> ` + studioHome + `/logs/triage.log 2>&1
# END_AGENT_STUDIO
`
	os.WriteFile(crontabFile, []byte(block), 0o644)

	output, err := runStudio(t, studioHome, "jobs", "list")
	if err != nil {
		t.Fatalf("jobs list: %s: %v", output, err)
	}

	if !strings.Contains(output, "triage") {
		t.Errorf("expected triage job in output, got:\n%s", output)
	}
}

func TestJobsList_jsonOutput(t *testing.T) {
	studioHome := setupJobsListHome(t)

	crontabFile := filepath.Join(studioHome, "fake-crontab.txt")
	block := `# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=scan
*/5 * * * *  ` + studioHome + `/bin/agent-studio-run scan >> ` + studioHome + `/logs/scan.log 2>&1
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
			if !r.Enabled {
				t.Error("scan should be enabled=true")
			}
			if r.Schedule == "" {
				t.Error("scan schedule should not be empty")
			}
			if r.Concurrency < 1 {
				t.Error("scan concurrency should be >= 1")
			}
		}
	}
	if !found {
		t.Errorf("scan not found in JSON output: %s", output)
	}
}
