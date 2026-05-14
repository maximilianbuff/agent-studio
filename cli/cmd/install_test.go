package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testEnv returns an env slice with AGENT_STUDIO_HOME and HOME pointed at
// isolated temp dirs so tests never touch the real ~/.agent-studio or
// ~/.claude/CLAUDE.md.
func testEnv(studioHome string) []string {
	// Give each invocation its own HOME so claudemd.Path() resolves inside tmp.
	fakeHome := filepath.Dir(studioHome) // parent of studioHome is fine
	env := os.Environ()
	filtered := env[:0]
	for _, e := range env {
		if !strings.HasPrefix(e, "HOME=") && !strings.HasPrefix(e, "AGENT_STUDIO_HOME=") {
			filtered = append(filtered, e)
		}
	}
	filtered = append(filtered,
		"AGENT_STUDIO_HOME="+studioHome,
		"HOME="+fakeHome,
	)
	return filtered
}

// runStudio builds the binary once per test run and invokes it with the
// given arguments, with AGENT_STUDIO_HOME and HOME pointed at temp dirs.
func runStudio(t *testing.T, studioHome string, args ...string) (string, error) {
	t.Helper()
	bin := buildBinary(t)

	cmd := exec.Command(bin, args...)
	cmd.Env = testEnv(studioHome)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runStudioCopy copies the binary to a throwaway path before running.
// Use this for uninstall tests where the command removes os.Executable().
func runStudioCopy(t *testing.T, studioHome string, args ...string) (string, error) {
	t.Helper()
	src := buildBinary(t)

	tmp := t.TempDir()
	dst := filepath.Join(tmp, "studio")

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("copy binary: %v", err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatalf("copy binary: %v", err)
	}

	cmd := exec.Command(dst, args...)
	cmd.Env = testEnv(studioHome)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var compiledBinary string

func buildBinary(t *testing.T) string {
	t.Helper()
	if compiledBinary != "" {
		return compiledBinary
	}

	_, thisFile, _, _ := runtime.Caller(0)
	moduleRoot := filepath.Dir(filepath.Dir(thisFile))

	tmp, err := os.MkdirTemp("", "studio-bin-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}

	bin := filepath.Join(tmp, "studio")
	out, err := exec.Command("go", "build", "-o", bin, moduleRoot).CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed:\n%s", string(out))
	}
	compiledBinary = bin
	return bin
}

// ---- studio install ----

func TestInstall_createsDirectoryStructure(t *testing.T) {
	studioHome := t.TempDir()

	output, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("studio install failed:\n%s\nerr: %v", output, err)
	}

	for _, d := range []string{"bin", "prompts", "logs"} {
		p := filepath.Join(studioHome, d)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("directory not created: %s (%v)", p, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", p)
		}
	}
}

func TestInstall_copiesDefaultFiles(t *testing.T) {
	studioHome := t.TempDir()

	output, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("studio install failed:\n%s\nerr: %v", output, err)
	}

	for _, f := range []string{
		".gitignore",
		"config.json",
		"prompts/scan.md",
		"prompts/worker.md",
		"bin/agent-studio-run",
	} {
		if _, err := os.Stat(filepath.Join(studioHome, f)); err != nil {
			t.Errorf("expected file not found: %s", f)
		}
	}
}

func TestInstall_runnerScriptIsExecutable(t *testing.T) {
	studioHome := t.TempDir()

	output, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("studio install failed:\n%s\nerr: %v", output, err)
	}

	info, err := os.Stat(filepath.Join(studioHome, "bin", "agent-studio-run"))
	if err != nil {
		t.Fatalf("runner script not found: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("runner script not executable: mode %o", info.Mode())
	}
}

func TestInstall_initialisesGitRepo(t *testing.T) {
	studioHome := t.TempDir()

	output, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("studio install failed:\n%s\nerr: %v", output, err)
	}

	if _, err := os.Stat(filepath.Join(studioHome, ".git")); err != nil {
		t.Errorf("git repo not initialised: .git not found")
	}
}

func TestInstall_createsGitCommit(t *testing.T) {
	studioHome := t.TempDir()

	output, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("studio install failed:\n%s\nerr: %v", output, err)
	}

	out, err := exec.Command("git", "-C", studioHome, "log", "--oneline").CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %s: %v", string(out), err)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Error("no commits found after install")
	}
}

func TestInstall_idempotent(t *testing.T) {
	studioHome := t.TempDir()

	out1, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("first install: %s: %v", out1, err)
	}

	// Modify config to prove it is not overwritten on second run.
	configPath := filepath.Join(studioHome, "config.json")
	original, _ := os.ReadFile(configPath)
	modified := append(original, '\n')
	os.WriteFile(configPath, modified, 0o644)

	out2, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("second install: %s: %v", out2, err)
	}

	if !strings.Contains(out2, "skip") {
		t.Error("second install should report skipping existing files")
	}
	after, _ := os.ReadFile(configPath)
	if !bytes.Equal(after, modified) {
		t.Error("config.json should not be overwritten on second install")
	}
}

func TestInstall_forceOverwrites(t *testing.T) {
	studioHome := t.TempDir()

	out1, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("first install: %s: %v", out1, err)
	}

	runner := filepath.Join(studioHome, "bin", "agent-studio-run")
	os.WriteFile(runner, []byte("corrupted"), 0o755)

	out2, err := runStudio(t, studioHome, "install", "--no-cron", "--force")
	if err != nil {
		t.Fatalf("force install: %s: %v", out2, err)
	}

	data, _ := os.ReadFile(runner)
	if string(data) == "corrupted" {
		t.Error("--force should overwrite existing files")
	}
	if !strings.Contains(string(data), "agent-studio-run") {
		t.Error("runner script not restored correctly after --force")
	}
}

func TestInstall_dryRunCreatesNoFiles(t *testing.T) {
	studioHome := t.TempDir()

	output, err := runStudio(t, studioHome, "install", "--dry-run", "--no-cron")
	if err != nil {
		t.Fatalf("dry-run: %s: %v", output, err)
	}

	entries, _ := os.ReadDir(studioHome)
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("dry-run created files: %v", names)
	}
}

func TestInstall_gitignoreContent(t *testing.T) {
	studioHome := t.TempDir()

	output, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("studio install: %s: %v", output, err)
	}

	data, err := os.ReadFile(filepath.Join(studioHome, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore not found: %v", err)
	}
	for _, pattern := range []string{"queue.json", "*.lock", "logs/"} {
		if !strings.Contains(string(data), pattern) {
			t.Errorf(".gitignore missing pattern %q", pattern)
		}
	}
}

// ---- studio uninstall ----

func TestUninstall_noErrorWhenNoCrontabBlock(t *testing.T) {
	studioHome := t.TempDir()

	// Use --yes to skip the interactive confirmation prompt.
	// Use runStudioCopy so the binary self-removal doesn't delete the shared test binary.
	output, err := runStudioCopy(t, studioHome, "uninstall", "--yes")
	if err != nil {
		t.Fatalf("studio uninstall failed:\n%s\nerr: %v", output, err)
	}
}

func TestUninstall_purgeDeletesDirectory(t *testing.T) {
	studioHome := t.TempDir()

	out1, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("install: %s: %v", out1, err)
	}

	out2, err := runStudioCopy(t, studioHome, "uninstall", "--purge", "--yes")
	if err != nil {
		t.Fatalf("uninstall --purge: %s: %v", out2, err)
	}

	if _, err := os.Stat(studioHome); !os.IsNotExist(err) {
		t.Errorf("%s still exists after --purge", studioHome)
	}
}

func TestUninstall_withoutPurgeKeepsDirectory(t *testing.T) {
	studioHome := t.TempDir()

	out1, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("install: %s: %v", out1, err)
	}

	// --yes skips the first confirmation; no --purge so data dir is kept.
	out2, err := runStudioCopy(t, studioHome, "uninstall", "--yes")
	if err != nil {
		t.Fatalf("uninstall: %s: %v", out2, err)
	}

	if _, err := os.Stat(studioHome); err != nil {
		t.Errorf("uninstall without --purge removed %s: %v", studioHome, err)
	}
}

// ---- runner script ----

func TestRunnerScript_passesShellSyntaxCheck(t *testing.T) {
	studioHome := t.TempDir()

	out, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("install: %s: %v", out, err)
	}

	runner := filepath.Join(studioHome, "bin", "agent-studio-run")
	checkOut, err := exec.Command("bash", "-n", runner).CombinedOutput()
	if err != nil {
		t.Errorf("runner script syntax error:\n%s", string(checkOut))
	}
}

func TestRunnerScript_exitsNonZeroOnMissingPrompt(t *testing.T) {
	studioHome := t.TempDir()

	out, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("install: %s: %v", out, err)
	}

	runner := filepath.Join(studioHome, "bin", "agent-studio-run")
	cmd := exec.Command("bash", runner, "nonexistent-job")
	cmd.Env = append(os.Environ(), "AGENT_STUDIO_HOME="+studioHome)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected non-zero exit when prompt is missing")
	}
	if !strings.Contains(string(output), "prompt not found") {
		t.Errorf("expected 'prompt not found' in output, got: %s", string(output))
	}
}

func TestRunnerScript_exitsNonZeroWithNoArg(t *testing.T) {
	studioHome := t.TempDir()

	out, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("install: %s: %v", out, err)
	}

	runner := filepath.Join(studioHome, "bin", "agent-studio-run")
	cmd := exec.Command("bash", runner)
	cmd.Env = append(os.Environ(), "AGENT_STUDIO_HOME="+studioHome)
	if _, err = cmd.CombinedOutput(); err == nil {
		t.Error("expected non-zero exit when no job argument given")
	}
}
