package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maximilianbuff/agent-studio/internal/config"
)

func setupIntervalHome(t *testing.T) string {
	t.Helper()
	studioHome := t.TempDir()
	out, err := runStudio(t, studioHome, "install", "--no-cron")
	if err != nil {
		t.Fatalf("install: %s: %v", out, err)
	}
	return studioHome
}

func TestWorkerInterval_defaultIsConfigurable(t *testing.T) {
	studioHome := setupIntervalHome(t)

	out, err := runStudio(t, studioHome, "config", "get", "worker_interval")
	if err != nil {
		t.Fatalf("config get worker_interval: %s: %v", out, err)
	}
	if strings.TrimSpace(out) != config.DefaultWorkerInterval {
		t.Errorf("expected %q, got %q", config.DefaultWorkerInterval, strings.TrimSpace(out))
	}
}

func TestScanInterval_defaultIsConfigurable(t *testing.T) {
	studioHome := setupIntervalHome(t)

	out, err := runStudio(t, studioHome, "config", "get", "scan_interval")
	if err != nil {
		t.Fatalf("config get scan_interval: %s: %v", out, err)
	}
	if strings.TrimSpace(out) != config.DefaultScanInterval {
		t.Errorf("expected %q, got %q", config.DefaultScanInterval, strings.TrimSpace(out))
	}
}

func TestWorkerInterval_setUpdatesConfig(t *testing.T) {
	studioHome := setupIntervalHome(t)

	custom := "*/15 * * * *"
	out, err := runStudio(t, studioHome, "config", "set", "worker_interval", custom)
	if err != nil {
		t.Fatalf("config set worker_interval: %s: %v", out, err)
	}

	out, err = runStudio(t, studioHome, "config", "get", "worker_interval")
	if err != nil {
		t.Fatalf("config get worker_interval after set: %s: %v", out, err)
	}
	if strings.TrimSpace(out) != custom {
		t.Errorf("expected %q, got %q", custom, strings.TrimSpace(out))
	}

	// Verify persisted to config.json via Jobs map.
	data, _ := os.ReadFile(filepath.Join(studioHome, "config.json"))
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	jobs, _ := raw["jobs"].(map[string]any)
	if jobs == nil {
		t.Fatal("jobs key missing from config.json")
	}
	worker, _ := jobs["worker"].(map[string]any)
	if worker == nil {
		t.Fatal("jobs.worker missing from config.json")
	}
	if worker["interval"] != custom {
		t.Errorf("jobs.worker.interval: expected %q, got %v", custom, worker["interval"])
	}
}

func TestScanInterval_setUpdatesConfig(t *testing.T) {
	studioHome := setupIntervalHome(t)

	custom := "0 */6 * * *"
	out, err := runStudio(t, studioHome, "config", "set", "scan_interval", custom)
	if err != nil {
		t.Fatalf("config set scan_interval: %s: %v", out, err)
	}

	out, err = runStudio(t, studioHome, "config", "get", "scan_interval")
	if err != nil {
		t.Fatalf("config get scan_interval after set: %s: %v", out, err)
	}
	if strings.TrimSpace(out) != custom {
		t.Errorf("expected %q, got %q", custom, strings.TrimSpace(out))
	}
}
