package home

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDir_usesEnvVar(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AGENT_STUDIO_HOME", tmp)
	if got := Dir(); got != tmp {
		t.Fatalf("Dir() = %q, want %q", got, tmp)
	}
}

func TestDir_defaultsToHomeDir(t *testing.T) {
	t.Setenv("AGENT_STUDIO_HOME", "")
	got := Dir()
	userHome, _ := os.UserHomeDir()
	want := filepath.Join(userHome, ".agent-studio")
	if got != want {
		t.Fatalf("Dir() = %q, want %q", got, want)
	}
}

func TestPath_joinsElements(t *testing.T) {
	t.Setenv("AGENT_STUDIO_HOME", "/tmp/test-studio")
	got := Path("prompts", "scan.md")
	want := "/tmp/test-studio/prompts/scan.md"
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}
