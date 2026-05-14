package prereqs

import (
	"fmt"
	"os/exec"
	"strings"
)

// Dep is a required external tool.
type Dep struct {
	Name string
	Hint string // install hint shown on missing
}

// Required lists tools that must be on PATH before studio install can run.
var Required = []Dep{
	{"git", "https://git-scm.com/"},
	{"gh", "https://cli.github.com/"},
	{"claude", "https://claude.ai/code"},
}

// Result reports whether a dependency was found.
type Result struct {
	Dep   Dep
	Path  string
	Found bool
}

// Check verifies that all given deps are on PATH.
// It uses lookPath to resolve binaries, allowing injection in tests.
func Check(deps []Dep, lookPath func(string) (string, error)) []Result {
	results := make([]Result, len(deps))
	for i, dep := range deps {
		p, err := lookPath(dep.Name)
		results[i] = Result{Dep: dep, Path: p, Found: err == nil}
	}
	return results
}

// CheckRequired runs Check against the Required list using exec.LookPath.
func CheckRequired() []Result {
	return Check(Required, exec.LookPath)
}

// Err returns a formatted error if any results are missing, nil otherwise.
func Err(results []Result) error {
	var missing []string
	for _, r := range results {
		if !r.Found {
			missing = append(missing, fmt.Sprintf("  %s  — not found (see %s)", r.Dep.Name, r.Dep.Hint))
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing required tools:\n%s", strings.Join(missing, "\n"))
}
