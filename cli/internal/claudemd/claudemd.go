// Package claudemd manages the AgentStudio block inside ~/.claude/CLAUDE.md.
// Claude Code loads that file in every session, so injecting a block there
// makes any agent running on this machine automatically aware of studio.
package claudemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	beginMarker = "<!-- BEGIN_AGENT_STUDIO -->"
	endMarker   = "<!-- END_AGENT_STUDIO -->"
)

// Path returns the path to the user-level CLAUDE.md file.
func Path() string {
	h, err := os.UserHomeDir()
	if err != nil {
		panic("claudemd: cannot determine home directory: " + err.Error())
	}
	return filepath.Join(h, ".claude", "CLAUDE.md")
}

// Block returns the Markdown block to inject, describing studio and its commands.
func Block(studioHome string) string {
	return fmt.Sprintf(`%s
## AgentStudio

Autonomous GitHub agent installed on this machine. Runs headless Claude sessions
via cron to scan repositories, triage issues, and open pull requests.

**Binary:** `+"`studio`"+` (on PATH)
**State directory:** `+"`%s`"+` (git repository — config, prompts, logs)

### When to use studio

- Before starting work in any repository: run `+"`studio queue show`"+` to check if
  AgentStudio is already working on something in that repo.
- To understand what the agent will do next: `+"`studio prompts show worker`"+`
- To check if a job is running: `+"`studio jobs list`"+`

### Key commands

`+"```"+`sh
studio jobs list                    # scheduled jobs, last run, status
studio jobs run <job>               # trigger a job immediately
studio jobs logs <job> --tail       # follow live output
studio jobs disable <job>           # pause without removing schedule

studio queue show                   # pending work queue (scored by priority)
studio queue clear                  # empty the queue

studio config show                  # repos, labels, scoring rules
studio config repos add owner/repo  # add a repository to scan
studio config set my_login <login>  # set GitHub login

studio prompts show scan            # see what the scan agent does
studio prompts show worker          # see what the worker agent does
studio prompts edit <job>           # edit a prompt (auto-commits)

studio history                      # audit trail of all config/prompt changes
studio rollback <sha>               # restore config/prompts to a prior state
`+"```"+`

### State layout

`+"```"+`
%s/
  config.json        ← repos, labels, scoring
  prompts/scan.md    ← scan agent instructions
  prompts/worker.md  ← worker agent instructions
  queue.json         ← current work queue (git-ignored)
  logs/              ← cron output (git-ignored)
`+"```"+`
%s`, beginMarker, studioHome, studioHome, endMarker)
}

// Inject writes the AgentStudio block into the CLAUDE.md file.
// Creates the file (and parent directory) if they do not exist.
// Replaces the existing block if one is already present.
func Inject(studioHome string) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	existing := ""
	if data, err := os.ReadFile(p); err == nil {
		existing = string(data)
	}

	block := Block(studioHome)
	updated := replaceBlock(existing, block)
	return os.WriteFile(p, []byte(updated), 0o644)
}

// Remove deletes the AgentStudio block from CLAUDE.md.
// No-op if the file does not exist or contains no block.
func Remove() error {
	p := Path()
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !HasBlock(string(data)) {
		return nil
	}
	updated := replaceBlock(string(data), "")
	return os.WriteFile(p, []byte(updated), 0o644)
}

// HasBlock reports whether content contains an AgentStudio block.
func HasBlock(content string) bool {
	return strings.Contains(content, beginMarker)
}

// replaceBlock replaces the AgentStudio block in content with newBlock.
// If no block exists, newBlock is appended. If newBlock is empty, the
// block is removed entirely.
func replaceBlock(content, newBlock string) string {
	lines := strings.Split(content, "\n")

	var before, after []string
	inBlock := false
	blockFound := false

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == beginMarker {
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
			before = append(before, l)
		} else {
			after = append(after, l)
		}
	}

	before = trimTrailingEmpty(before)
	after = trimLeadingEmpty(after)

	if newBlock == "" {
		// Removal: just stitch before + after back together.
		parts := []string{}
		if len(before) > 0 {
			parts = append(parts, strings.Join(before, "\n"))
		}
		if len(after) > 0 {
			parts = append(parts, strings.Join(after, "\n"))
		}
		result := strings.Join(parts, "\n")
		if result != "" && !strings.HasSuffix(result, "\n") {
			result += "\n"
		}
		return result
	}

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
