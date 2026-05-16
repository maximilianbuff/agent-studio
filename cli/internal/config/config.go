// Package config reads and writes ~/.agent-studio/config.json.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/maximilianbuff/agent-studio/internal/home"
)

const (
	DefaultScanInterval   = "*/5 * * * *"
	DefaultWorkerInterval = "*/10 * * * *"
)

// JobConfig holds per-job scheduling and concurrency settings.
type JobConfig struct {
	Interval    string `json:"interval,omitempty"`
	Concurrency int    `json:"concurrency,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
}

// Config is the full studio configuration.
type Config struct {
	MyLogin         string                `json:"my_login"`
	Repos           []RepoEntry           `json:"repos"`
	Labels          map[string]int        `json:"labels"`
	Keywords        map[string]int        `json:"keywords"`
	SkipLabels      []string              `json:"skip_labels"`
	MinScore        int                   `json:"min_score"`
	Jobs            map[string]JobConfig  `json:"jobs,omitempty"`
	AuthMode        string                `json:"auth_mode,omitempty"`
	AnthropicAPIKey string                `json:"anthropic_api_key,omitempty"`
}

// JobInterval returns the cron schedule for job, falling back to built-in defaults.
func (c Config) JobInterval(job string) string {
	if c.Jobs != nil {
		if j, ok := c.Jobs[job]; ok && j.Interval != "" {
			return j.Interval
		}
	}
	switch job {
	case "scan":
		return DefaultScanInterval
	case "worker":
		return DefaultWorkerInterval
	}
	return DefaultWorkerInterval
}

// JobConcurrency returns the max concurrent instances for job (default 1).
func (c Config) JobConcurrency(job string) int {
	if c.Jobs != nil {
		if j, ok := c.Jobs[job]; ok && j.Concurrency > 0 {
			return j.Concurrency
		}
	}
	return 1
}

// JobDisabled reports whether job is disabled in config.
func (c Config) JobDisabled(job string) bool {
	if c.Jobs != nil {
		if j, ok := c.Jobs[job]; ok {
			return j.Disabled
		}
	}
	return false
}

// RepoEntry is a repository with an optional priority multiplier.
type RepoEntry struct {
	Repo     string  `json:"repo"`
	Priority float64 `json:"priority,omitempty"`
	Disabled bool    `json:"disabled,omitempty"`
}

// Path returns the path to config.json.
func Path() string {
	return home.Path("config.json")
}

// Load reads and parses config.json. Returns a zero Config if the file is absent.
func Load() (Config, error) {
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", Path(), err)
	}
	if c.Labels == nil {
		c.Labels = map[string]int{}
	}
	if c.Keywords == nil {
		c.Keywords = map[string]int{}
	}
	return c, nil
}

// Save writes c back to config.json with a trailing newline.
// Uses mode 0600 when an API key is present to protect the secret.
func Save(c Config) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	mode := os.FileMode(0o644)
	if c.AnthropicAPIKey != "" {
		mode = 0o600
	}
	return os.WriteFile(p, data, mode)
}

// SetJob updates interval and/or concurrency for a named job.
func SetJob(c *Config, job, interval string, concurrency int) {
	if c.Jobs == nil {
		c.Jobs = map[string]JobConfig{}
	}
	j := c.Jobs[job]
	if interval != "" {
		j.Interval = interval
	}
	if concurrency > 0 {
		j.Concurrency = concurrency
	}
	c.Jobs[job] = j
}

// Set applies a key=value update to the config.
func Set(c *Config, key, value string) error {
	switch {
	case key == "my_login":
		c.MyLogin = value
	case key == "min_score":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("min_score must be an integer: %w", err)
		}
		c.MinScore = n
	case key == "auth_mode":
		if value != "subscription" && value != "api_key" {
			return fmt.Errorf("auth_mode must be \"subscription\" or \"api_key\"")
		}
		c.AuthMode = value
	case key == "anthropic_api_key":
		c.AnthropicAPIKey = value
	case strings.HasPrefix(key, "labels."):
		label := strings.TrimPrefix(key, "labels.")
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("label score must be an integer: %w", err)
		}
		c.Labels[label] = n
	case strings.HasPrefix(key, "keywords."):
		kw := strings.TrimPrefix(key, "keywords.")
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("keyword score must be an integer: %w", err)
		}
		c.Keywords[kw] = n
	default:
		return fmt.Errorf("unknown key %q — valid keys: my_login, min_score, auth_mode, anthropic_api_key, labels.<label>, keywords.<word>", key)
	}
	return nil
}

// RepoAdd adds a repo to the list. Priority defaults to 1.0 if <= 0.
func RepoAdd(c *Config, repo string, priority float64) error {
	if !strings.Contains(repo, "/") {
		return fmt.Errorf("repo must be in owner/repo format")
	}
	for _, r := range c.Repos {
		if r.Repo == repo {
			return fmt.Errorf("repo %q already in list", repo)
		}
	}
	p := priority
	if p <= 0 {
		p = 1.0
	}
	c.Repos = append(c.Repos, RepoEntry{Repo: repo, Priority: p})
	return nil
}

// RepoDisable marks a repo as disabled (skipped by the scan agent).
func RepoDisable(c *Config, repo string) error {
	idx := slices.IndexFunc(c.Repos, func(r RepoEntry) bool { return r.Repo == repo })
	if idx < 0 {
		return fmt.Errorf("repo %q not in list", repo)
	}
	c.Repos[idx].Disabled = true
	return nil
}

// RepoEnable clears the disabled flag on a repo.
func RepoEnable(c *Config, repo string) error {
	idx := slices.IndexFunc(c.Repos, func(r RepoEntry) bool { return r.Repo == repo })
	if idx < 0 {
		return fmt.Errorf("repo %q not in list", repo)
	}
	c.Repos[idx].Disabled = false
	return nil
}

// RepoRemove removes a repo from the list.
func RepoRemove(c *Config, repo string) error {
	idx := slices.IndexFunc(c.Repos, func(r RepoEntry) bool { return r.Repo == repo })
	if idx < 0 {
		return fmt.Errorf("repo %q not in list", repo)
	}
	c.Repos = slices.Delete(c.Repos, idx, idx+1)
	return nil
}
