// Package crontab manages the AgentStudio block inside the user's crontab.
// The block is delimited by sentinel comments and is the only part this package
// ever reads or writes; all other crontab entries are left untouched.
//
// Block format:
//
//	# BEGIN_AGENT_STUDIO — do not edit this block manually
//	# AGENT_STUDIO_JOB=scan
//	7 */2 * * *  ~/.agent-studio/bin/agent-studio-run scan >> ~/.agent-studio/logs/scan.log 2>&1
//	# AGENT_STUDIO_JOB=worker
//	*/2 * * * *  ~/.agent-studio/bin/agent-studio-run worker >> ~/.agent-studio/logs/worker.log 2>&1
//	# END_AGENT_STUDIO
//
// Disabled entries have their cron line prefixed with '#'.
package crontab

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

const (
	beginMarker = "# BEGIN_AGENT_STUDIO"
	endMarker   = "# END_AGENT_STUDIO"
	jobPrefix   = "# AGENT_STUDIO_JOB="
)

// Entry represents a single scheduled AgentStudio job.
type Entry struct {
	Job      string
	Schedule string
	Enabled  bool
}

// ParseBlock extracts AgentStudio entries from a full crontab string.
// Returns an empty slice if the block is absent.
func ParseBlock(crontabText string) []Entry {
	lines := strings.Split(crontabText, "\n")

	inBlock := false
	var entries []Entry
	var pendingJob string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, beginMarker) {
			inBlock = true
			continue
		}
		if trimmed == endMarker {
			break
		}
		if !inBlock {
			continue
		}

		if strings.HasPrefix(trimmed, jobPrefix) {
			pendingJob = strings.TrimPrefix(trimmed, jobPrefix)
			continue
		}

		if pendingJob == "" {
			continue
		}

		enabled := true
		cronLine := trimmed
		if strings.HasPrefix(cronLine, "#") {
			enabled = false
			cronLine = strings.TrimSpace(strings.TrimPrefix(cronLine, "#"))
		}

		schedule := extractSchedule(cronLine)
		entries = append(entries, Entry{
			Job:      pendingJob,
			Schedule: schedule,
			Enabled:  enabled,
		})
		pendingJob = ""
	}

	return entries
}

// FormatBlock renders the AgentStudio crontab block for the given entries.
func FormatBlock(entries []Entry, studioHome string) string {
	var b strings.Builder
	b.WriteString(beginMarker + " — do not edit this block manually\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "%s%s\n", jobPrefix, e.Job)
		line := fmt.Sprintf(
			"%s  %s/bin/agent-studio-run %s >> %s/logs/%s.log 2>&1",
			e.Schedule, studioHome, e.Job, studioHome, e.Job,
		)
		if !e.Enabled {
			line = "# " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(endMarker + "\n")
	return b.String()
}

// ReplaceBlock replaces the AgentStudio block inside fullCrontab with newBlock.
// If no block exists, newBlock is appended.
func ReplaceBlock(fullCrontab, newBlock string) string {
	lines := strings.Split(fullCrontab, "\n")

	var before, after []string
	inBlock := false
	blockFound := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, beginMarker) {
			inBlock = true
			blockFound = true
			continue
		}
		if trimmed == endMarker {
			inBlock = false
			continue
		}
		if inBlock {
			continue
		}
		if !blockFound {
			before = append(before, line)
		} else {
			after = append(after, line)
		}
	}

	// Remove trailing empty lines from before and leading empty lines from after
	// to avoid accumulating blank lines around the block on repeated writes.
	before = trimTrailingEmpty(before)
	after = trimLeadingEmpty(after)

	var parts []string
	if len(before) > 0 {
		parts = append(parts, strings.Join(before, "\n"))
	}
	parts = append(parts, strings.TrimRight(newBlock, "\n"))
	if len(after) > 0 {
		parts = append(parts, strings.Join(after, "\n"))
	}

	result := strings.Join(parts, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result
}

// RemoveBlock removes the AgentStudio block from fullCrontab entirely.
func RemoveBlock(fullCrontab string) string {
	return ReplaceBlock(fullCrontab, "")
}

// HasBlock reports whether fullCrontab contains an AgentStudio block.
func HasBlock(fullCrontab string) bool {
	return strings.Contains(fullCrontab, beginMarker)
}

// Read returns the current user crontab contents.
// An empty crontab (exit 1 with "no crontab for user") is treated as "".
func Read(runCmd func(string, ...string) ([]byte, error)) (string, error) {
	out, err := runCmd("crontab", "-l")
	if err != nil {
		// crontab -l exits 1 when there is no crontab — treat as empty.
		if isNoCrontabErr(string(out)) || isNoCrontabErr(err.Error()) {
			return "", nil
		}
		return "", fmt.Errorf("crontab -l: %w", err)
	}
	return string(out), nil
}

// Write installs content as the user's crontab.
func Write(content string, runCmdStdin func(string, string, ...string) error) error {
	return runCmdStdin(content, "crontab", "-")
}

// DefaultRunCmd shells out via exec.Command.
func DefaultRunCmd(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// DefaultRunCmdStdin runs a command with the given string on stdin.
func DefaultRunCmdStdin(stdin, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s: %w", name, string(out), err)
	}
	return nil
}

// GetEntries reads the crontab and parses the AgentStudio block.
func GetEntries(runCmd func(string, ...string) ([]byte, error)) ([]Entry, error) {
	text, err := Read(runCmd)
	if err != nil {
		return nil, err
	}
	return ParseBlock(text), nil
}

// SetEntries writes the AgentStudio block with the given entries.
func SetEntries(
	entries []Entry,
	studioHome string,
	runCmd func(string, ...string) ([]byte, error),
	runCmdStdin func(string, string, ...string) error,
) error {
	text, err := Read(runCmd)
	if err != nil {
		return err
	}
	block := FormatBlock(entries, studioHome)
	updated := ReplaceBlock(text, block)
	return Write(updated, runCmdStdin)
}

// DeleteBlock removes the AgentStudio block from the crontab entirely.
func DeleteBlock(
	runCmd func(string, ...string) ([]byte, error),
	runCmdStdin func(string, string, ...string) error,
) error {
	text, err := Read(runCmd)
	if err != nil {
		return err
	}
	if !HasBlock(text) {
		return nil
	}
	updated := RemoveBlock(text)
	return Write(updated, runCmdStdin)
}

// extractSchedule parses the cron schedule expression (first 5 fields) from a cron line.
func extractSchedule(cronLine string) string {
	fields := strings.Fields(cronLine)
	if len(fields) < 5 {
		return cronLine
	}
	return strings.Join(fields[:5], " ")
}

func isNoCrontabErr(s string) bool {
	return strings.Contains(s, "no crontab for") || strings.Contains(s, "no crontab")
}

func trimTrailingEmpty(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func trimLeadingEmpty(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	return lines
}
