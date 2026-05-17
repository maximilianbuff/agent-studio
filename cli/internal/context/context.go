// Package context manages per-repo persistent context files at
// ~/.agent-studio/context/<owner>-<repo>.md.
package context

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/maximilianbuff/agent-studio/internal/home"
)

// Dir returns the context directory path.
func Dir() string {
	return home.Path("context")
}

// filename converts "owner/repo" to "owner-repo.md".
func filename(repo string) string {
	return strings.ReplaceAll(repo, "/", "-") + ".md"
}

// Path returns the context file path for the given repo ("owner/repo").
func Path(repo string) string {
	return filepath.Join(Dir(), filename(repo))
}

// Read returns the context file contents for repo. Returns ("", nil) if absent.
func Read(repo string) (string, error) {
	data, err := os.ReadFile(Path(repo))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Write writes content to the context file for repo, creating the directory if needed.
func Write(repo, content string) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(Path(repo), []byte(content), 0o644)
}

// Clear removes the context file for repo. Returns nil if absent.
func Clear(repo string) error {
	err := os.Remove(Path(repo))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// List returns all repos with a context file (as "owner/repo" strings).
func List() ([]string, error) {
	entries, err := os.ReadDir(Dir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		// Convert first "-" separator to "/" — but owner/repo both may contain "-".
		// We store as "owner-repo.md" where the first "-" after a non-"-" run
		// separating owner from repo. Use the file as-is and present as stored name.
		repos = append(repos, name)
	}
	return repos, nil
}
