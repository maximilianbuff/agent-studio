// Package queue reads and writes ~/.agent-studio/queue.json.
package queue

import (
	"encoding/json"
	"os"

	"github.com/maximilianbuff/agent-studio/internal/home"
)

// Item represents a single work item in the queue.
type Item struct {
	Repo    string `json:"repo"`
	Issue   int    `json:"issue"`
	Title   string `json:"title"`
	Score   int    `json:"score"`
	Claimed bool   `json:"claimed,omitempty"`
}

// Path returns the path to queue.json.
func Path() string {
	return home.Path("queue.json")
}

// Load reads and parses queue.json. Returns an empty slice if absent.
func Load() ([]Item, error) {
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// Clear removes queue.json entirely.
func Clear() error {
	err := os.Remove(Path())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
