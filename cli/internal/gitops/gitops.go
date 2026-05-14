package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsRepo reports whether dir is inside a git repository.
func IsRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// Init runs git init in dir.
func Init(dir string) error {
	return run(dir, "git", "init", "-b", "main")
}

// ConfigUser sets a local git user name and email so commits work
// even on machines with no global git config.
func ConfigUser(dir string) error {
	if err := run(dir, "git", "config", "user.name", "agent-studio"); err != nil {
		return err
	}
	return run(dir, "git", "config", "user.email", "agent-studio@localhost")
}

// AddAll stages all changes in dir.
func AddAll(dir string) error {
	return run(dir, "git", "add", "-A")
}

// Commit creates a commit in dir with the given message.
// Returns nil without error if there is nothing to commit.
func Commit(dir, msg string) error {
	cmd := exec.Command("git", "-C", dir, "commit", "-m", msg)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=agent-studio",
		"GIT_AUTHOR_EMAIL=agent-studio@localhost",
		"GIT_COMMITTER_NAME=agent-studio",
		"GIT_COMMITTER_EMAIL=agent-studio@localhost",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isNothingToCommit(string(out)) {
			return nil
		}
		return fmt.Errorf("git commit: %s: %w", string(out), err)
	}
	return nil
}

// Log returns up to n one-line git log entries for dir.
func Log(dir string, n int) ([]string, error) {
	args := []string{"-C", dir, "log", "--oneline"}
	if n > 0 {
		args = append(args, fmt.Sprintf("-%d", n))
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git log: %s: %w", string(out), err)
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

// Diff returns changes between ref and the working tree for dir.
// Use ref="HEAD" for uncommitted changes.
func Diff(dir, ref string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "diff", ref).CombinedOutput()
	if err != nil {
		return string(out), nil // diff exits 1 when changes exist — still useful
	}
	return string(out), nil
}

// Rollback creates a revert commit that undoes all changes since sha.
func Rollback(dir, sha string) error {
	cmd := exec.Command("git", "-C", dir, "revert", "--no-commit", sha+"..HEAD")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=agent-studio",
		"GIT_AUTHOR_EMAIL=agent-studio@localhost",
		"GIT_COMMITTER_NAME=agent-studio",
		"GIT_COMMITTER_EMAIL=agent-studio@localhost",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git revert: %s: %w", string(out), err)
	}
	return Commit(dir, fmt.Sprintf("rollback: revert to %s", sha))
}

// RemoteSet sets (or replaces) the origin remote.
func RemoteSet(dir, url string) error {
	// Remove existing origin if present.
	exec.Command("git", "-C", dir, "remote", "remove", "origin").Run() //nolint:errcheck
	return run(dir, "git", "remote", "add", "origin", url)
}

// RemoteGet returns the current origin URL, or "" if none.
func RemoteGet(dir string) string {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").CombinedOutput()
	if err != nil {
		return ""
	}
	return filepath.Clean(string(out))
}

// Push pushes the current branch to origin.
func Push(dir string) error {
	out, err := exec.Command("git", "-C", dir, "push", "--set-upstream", "origin", "HEAD").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push: %s: %w", string(out), err)
	}
	return nil
}

// Pull fetches and rebases from origin.
func Pull(dir string) error {
	out, err := exec.Command("git", "-C", dir, "pull", "--rebase").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull: %s: %w", string(out), err)
	}
	return nil
}

func run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %s: %w", name, args, string(out), err)
	}
	return nil
}

func isNothingToCommit(output string) bool {
	return contains(output, "nothing to commit") || contains(output, "nothing added to commit")
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
