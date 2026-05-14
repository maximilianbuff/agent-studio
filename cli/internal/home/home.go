package home

import (
	"os"
	"path/filepath"
)

// Dir returns the AgentStudio state directory.
// Respects AGENT_STUDIO_HOME if set, otherwise ~/.agent-studio.
func Dir() string {
	if h := os.Getenv("AGENT_STUDIO_HOME"); h != "" {
		return h
	}
	h, err := os.UserHomeDir()
	if err != nil {
		panic("agent-studio: cannot determine home directory: " + err.Error())
	}
	return filepath.Join(h, ".agent-studio")
}

// Path joins the state directory with the given path components.
func Path(elem ...string) string {
	parts := append([]string{Dir()}, elem...)
	return filepath.Join(parts...)
}
