package prereqs

import (
	"errors"
	"testing"
)

func fakeLookPath(available map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if p, ok := available[name]; ok {
			return p, nil
		}
		return "", errors.New("not found")
	}
}

func TestCheck_allFound(t *testing.T) {
	deps := []Dep{
		{Name: "git", Hint: "https://git-scm.com/"},
		{Name: "gh", Hint: "https://cli.github.com/"},
	}
	look := fakeLookPath(map[string]string{"git": "/usr/bin/git", "gh": "/usr/bin/gh"})
	results := Check(deps, look)
	for _, r := range results {
		if !r.Found {
			t.Errorf("%s: expected found=true", r.Dep.Name)
		}
	}
	if err := Err(results); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}

func TestCheck_someMissing(t *testing.T) {
	deps := []Dep{
		{Name: "git", Hint: "https://git-scm.com/"},
		{Name: "claude", Hint: "https://claude.ai/code"},
	}
	look := fakeLookPath(map[string]string{"git": "/usr/bin/git"})
	results := Check(deps, look)

	if !results[0].Found {
		t.Error("git: expected found=true")
	}
	if results[1].Found {
		t.Error("claude: expected found=false")
	}

	err := Err(results)
	if err == nil {
		t.Fatal("Err() = nil, want error")
	}
	got := err.Error()
	if !contains(got, "claude") {
		t.Errorf("error message missing 'claude': %s", got)
	}
	if contains(got, "git") {
		t.Errorf("error message should not mention found tool 'git': %s", got)
	}
}

func TestCheck_allMissing(t *testing.T) {
	deps := []Dep{
		{Name: "git", Hint: "https://git-scm.com/"},
		{Name: "gh", Hint: "https://cli.github.com/"},
		{Name: "claude", Hint: "https://claude.ai/code"},
	}
	look := fakeLookPath(map[string]string{})
	results := Check(deps, look)
	for _, r := range results {
		if r.Found {
			t.Errorf("%s: expected found=false", r.Dep.Name)
		}
	}
	err := Err(results)
	if err == nil {
		t.Fatal("Err() = nil, want error")
	}
}

func TestErr_nilWhenAllFound(t *testing.T) {
	results := []Result{
		{Dep: Dep{Name: "git"}, Path: "/usr/bin/git", Found: true},
	}
	if err := Err(results); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
