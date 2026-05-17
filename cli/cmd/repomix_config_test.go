package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupRepomixHome(t *testing.T) string {
	t.Helper()
	studioHome := t.TempDir()
	out, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("install: %s: %v", out, err)
	}
	return studioHome
}

// --- repomix_enabled config ---

func TestRepomixEnabled_defaultTrue(t *testing.T) {
	studioHome := setupRepomixHome(t)

	out, err := runStudio(t, studioHome, "config", "get", "repomix_enabled")
	if err != nil {
		t.Fatalf("config get repomix_enabled: %s: %v", out, err)
	}
	if strings.TrimSpace(out) != "true" {
		t.Errorf("expected default repomix_enabled=true, got %q", strings.TrimSpace(out))
	}
}

func TestRepomixEnabled_setFalse(t *testing.T) {
	studioHome := setupRepomixHome(t)

	out, err := runStudio(t, studioHome, "config", "set", "repomix_enabled", "false")
	if err != nil {
		t.Fatalf("config set repomix_enabled false: %s: %v", out, err)
	}

	out, err = runStudio(t, studioHome, "config", "get", "repomix_enabled")
	if err != nil {
		t.Fatalf("config get repomix_enabled after set: %s: %v", out, err)
	}
	if strings.TrimSpace(out) != "false" {
		t.Errorf("expected repomix_enabled=false after set, got %q", strings.TrimSpace(out))
	}
}

func TestRepomixEnabled_setTrue(t *testing.T) {
	studioHome := setupRepomixHome(t)

	// Disable first, then re-enable.
	if out, err := runStudio(t, studioHome, "config", "set", "repomix_enabled", "false"); err != nil {
		t.Fatalf("set false: %s: %v", out, err)
	}
	if out, err := runStudio(t, studioHome, "config", "set", "repomix_enabled", "true"); err != nil {
		t.Fatalf("set true: %s: %v", out, err)
	}

	out, err := runStudio(t, studioHome, "config", "get", "repomix_enabled")
	if err != nil {
		t.Fatalf("config get repomix_enabled: %s: %v", out, err)
	}
	if strings.TrimSpace(out) != "true" {
		t.Errorf("expected repomix_enabled=true after re-enable, got %q", strings.TrimSpace(out))
	}
}

func TestRepomixEnabled_persistedInConfigJSON(t *testing.T) {
	studioHome := setupRepomixHome(t)

	if out, err := runStudio(t, studioHome, "config", "set", "repomix_enabled", "false"); err != nil {
		t.Fatalf("set: %s: %v", out, err)
	}

	data, err := os.ReadFile(filepath.Join(studioHome, "config.json"))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse config.json: %v", err)
	}
	if v, ok := raw["repomix_enabled"]; !ok || v != false {
		t.Errorf("expected repomix_enabled=false in config.json, got %v", v)
	}
}

func TestRepomixEnabled_invalidValue(t *testing.T) {
	studioHome := setupRepomixHome(t)

	out, err := runStudio(t, studioHome, "config", "set", "repomix_enabled", "yes-please")
	if err == nil {
		t.Fatalf("expected error for invalid value, got: %s", out)
	}
}

// --- studio queue peek ---

func TestQueuePeek_emptyQueue(t *testing.T) {
	studioHome := setupRepomixHome(t)

	out, err := runStudio(t, studioHome, "queue", "peek")
	if err != nil {
		t.Fatalf("queue peek on empty queue: %s: %v", out, err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output for empty queue, got %q", out)
	}
}

func TestQueuePeek_returnsFirstRepo(t *testing.T) {
	studioHome := setupRepomixHome(t)

	queue := `{
  "scanned_at": "2026-05-17T00:00:00Z",
  "items": [
    {"type": "issue", "repo": "owner/first-repo", "number": 1, "title": "First", "score": 10},
    {"type": "issue", "repo": "owner/second-repo", "number": 2, "title": "Second", "score": 5}
  ]
}`
	if err := os.WriteFile(filepath.Join(studioHome, "queue.json"), []byte(queue), 0o644); err != nil {
		t.Fatalf("write queue.json: %v", err)
	}

	out, err := runStudio(t, studioHome, "queue", "peek")
	if err != nil {
		t.Fatalf("queue peek: %s: %v", out, err)
	}
	if strings.TrimSpace(out) != "owner/first-repo" {
		t.Errorf("expected first repo, got %q", strings.TrimSpace(out))
	}
}
