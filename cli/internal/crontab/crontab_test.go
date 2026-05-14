package crontab

import (
	"strings"
	"testing"
)

const studioHome = "/home/user/.agent-studio"

// ---- ParseBlock ----

func TestParseBlock_empty(t *testing.T) {
	entries := ParseBlock("")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseBlock_noBlock(t *testing.T) {
	crontab := "# some other crontab\n0 * * * * /usr/bin/backup\n"
	entries := ParseBlock(crontab)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseBlock_twoEnabled(t *testing.T) {
	crontab := `# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=scan
7 */2 * * *  /home/user/.agent-studio/bin/agent-studio-run scan >> /home/user/.agent-studio/logs/scan.log 2>&1
# AGENT_STUDIO_JOB=worker
*/2 * * * *  /home/user/.agent-studio/bin/agent-studio-run worker >> /home/user/.agent-studio/logs/worker.log 2>&1
# END_AGENT_STUDIO
`
	entries := ParseBlock(crontab)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Job != "scan" || entries[0].Schedule != "7 */2 * * *" || !entries[0].Enabled {
		t.Errorf("scan entry wrong: %+v", entries[0])
	}
	if entries[1].Job != "worker" || entries[1].Schedule != "*/2 * * * *" || !entries[1].Enabled {
		t.Errorf("worker entry wrong: %+v", entries[1])
	}
}

func TestParseBlock_disabledEntry(t *testing.T) {
	crontab := `# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=scan
# 7 */2 * * *  /home/user/.agent-studio/bin/agent-studio-run scan >> /home/user/.agent-studio/logs/scan.log 2>&1
# END_AGENT_STUDIO
`
	entries := ParseBlock(crontab)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Enabled {
		t.Error("expected Enabled=false for commented-out entry")
	}
}

func TestParseBlock_preservesOtherLines(t *testing.T) {
	crontab := `0 * * * * /usr/bin/backup
# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=scan
7 */2 * * *  /home/user/.agent-studio/bin/agent-studio-run scan >> /home/user/.agent-studio/logs/scan.log 2>&1
# END_AGENT_STUDIO
30 * * * * /usr/bin/cleanup
`
	entries := ParseBlock(crontab)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

// ---- FormatBlock ----

func TestFormatBlock_enabledEntries(t *testing.T) {
	entries := []Entry{
		{Job: "scan", Schedule: "7 */2 * * *", Enabled: true},
		{Job: "worker", Schedule: "*/2 * * * *", Enabled: true},
	}
	block := FormatBlock(entries, studioHome)

	if !strings.Contains(block, beginMarker) {
		t.Error("missing begin marker")
	}
	if !strings.Contains(block, endMarker) {
		t.Error("missing end marker")
	}
	if !strings.Contains(block, "# AGENT_STUDIO_JOB=scan") {
		t.Error("missing scan job comment")
	}
	if !strings.Contains(block, "7 */2 * * *") {
		t.Error("missing scan schedule")
	}
	if !strings.Contains(block, "agent-studio-run scan") {
		t.Error("missing scan runner invocation")
	}
}

func TestFormatBlock_disabledEntry(t *testing.T) {
	entries := []Entry{
		{Job: "scan", Schedule: "7 */2 * * *", Enabled: false},
	}
	block := FormatBlock(entries, studioHome)
	lines := strings.Split(block, "\n")

	var cronLine string
	for _, l := range lines {
		if strings.Contains(l, "agent-studio-run scan") {
			cronLine = l
			break
		}
	}
	if cronLine == "" {
		t.Fatal("cron line not found in block")
	}
	if !strings.HasPrefix(strings.TrimSpace(cronLine), "#") {
		t.Errorf("disabled entry should be commented out, got: %q", cronLine)
	}
}

func TestFormatBlock_roundTrip(t *testing.T) {
	original := []Entry{
		{Job: "scan", Schedule: "7 */2 * * *", Enabled: true},
		{Job: "worker", Schedule: "*/2 * * * *", Enabled: false},
		{Job: "triage", Schedule: "0 9 * * *", Enabled: true},
	}
	block := FormatBlock(original, studioHome)
	parsed := ParseBlock(block)

	if len(parsed) != len(original) {
		t.Fatalf("round-trip: got %d entries, want %d", len(parsed), len(original))
	}
	for i, want := range original {
		got := parsed[i]
		if got.Job != want.Job || got.Schedule != want.Schedule || got.Enabled != want.Enabled {
			t.Errorf("entry %d: got %+v, want %+v", i, got, want)
		}
	}
}

// ---- ReplaceBlock ----

func TestReplaceBlock_insertsWhenAbsent(t *testing.T) {
	existing := "0 * * * * /usr/bin/backup\n"
	block := "# BEGIN_AGENT_STUDIO\n# END_AGENT_STUDIO\n"
	result := ReplaceBlock(existing, block)

	if !strings.Contains(result, "BEGIN_AGENT_STUDIO") {
		t.Error("block not inserted")
	}
	if !strings.Contains(result, "/usr/bin/backup") {
		t.Error("existing entry lost")
	}
}

func TestReplaceBlock_replacesExisting(t *testing.T) {
	existing := `0 * * * * /usr/bin/backup
# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=scan
7 */2 * * *  old-content
# END_AGENT_STUDIO
30 * * * * /usr/bin/cleanup
`
	newBlock := "# BEGIN_AGENT_STUDIO\nnew-content\n# END_AGENT_STUDIO\n"
	result := ReplaceBlock(existing, newBlock)

	if strings.Contains(result, "old-content") {
		t.Error("old block content should be gone")
	}
	if !strings.Contains(result, "new-content") {
		t.Error("new block content missing")
	}
	if !strings.Contains(result, "/usr/bin/backup") {
		t.Error("pre-block entry lost")
	}
	if !strings.Contains(result, "/usr/bin/cleanup") {
		t.Error("post-block entry lost")
	}
}

func TestReplaceBlock_idempotent(t *testing.T) {
	entries := []Entry{
		{Job: "scan", Schedule: "7 */2 * * *", Enabled: true},
	}
	block := FormatBlock(entries, studioHome)
	base := "0 * * * * /usr/bin/backup\n"

	first := ReplaceBlock(base, block)
	second := ReplaceBlock(first, block)

	if first != second {
		t.Errorf("ReplaceBlock is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// ---- RemoveBlock ----

func TestRemoveBlock_removesBlock(t *testing.T) {
	crontab := `0 * * * * /usr/bin/backup
# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=scan
7 */2 * * *  ~/.agent-studio/bin/agent-studio-run scan >> ~/.agent-studio/logs/scan.log 2>&1
# END_AGENT_STUDIO
30 * * * * /usr/bin/cleanup
`
	result := RemoveBlock(crontab)

	if strings.Contains(result, "BEGIN_AGENT_STUDIO") {
		t.Error("block should be removed")
	}
	if !strings.Contains(result, "/usr/bin/backup") {
		t.Error("pre-block entry lost")
	}
	if !strings.Contains(result, "/usr/bin/cleanup") {
		t.Error("post-block entry lost")
	}
}

func TestRemoveBlock_noOpWhenAbsent(t *testing.T) {
	crontab := "0 * * * * /usr/bin/backup\n"
	result := RemoveBlock(crontab)
	if result != crontab {
		t.Errorf("RemoveBlock changed crontab with no block:\ngot:  %q\nwant: %q", result, crontab)
	}
}

// ---- HasBlock ----

func TestHasBlock(t *testing.T) {
	if HasBlock("no block here") {
		t.Error("HasBlock should be false")
	}
	if !HasBlock("# BEGIN_AGENT_STUDIO\nstuff\n# END_AGENT_STUDIO\n") {
		t.Error("HasBlock should be true")
	}
}

// ---- Read (with injected runner) ----

func TestRead_emptyCrontab(t *testing.T) {
	run := func(name string, args ...string) ([]byte, error) {
		return []byte("no crontab for user"), nil
	}
	text, err := Read(run)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	_ = text // empty or the raw "no crontab" message — both are acceptable
}

func TestRead_returnsCrontabContent(t *testing.T) {
	want := "0 * * * * /usr/bin/backup\n"
	run := func(name string, args ...string) ([]byte, error) {
		return []byte(want), nil
	}
	got, err := Read(run)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if got != want {
		t.Errorf("Read() = %q, want %q", got, want)
	}
}
