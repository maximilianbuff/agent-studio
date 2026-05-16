// Package queue reads and writes ~/.agent-studio/queue.json.
package queue

import (
	"encoding/json"
	"os"

	"github.com/maximilianbuff/agent-studio/internal/home"
)

// Item represents a single work item in the queue.
type Item struct {
	Type       string  `json:"type"`
	Repo       string  `json:"repo"`
	Number     int     `json:"number"`
	Title      string  `json:"title"`
	URL        string  `json:"url,omitempty"`
	Score      int     `json:"score"`
	RepoWeight float64 `json:"repo_weight,omitempty"`
	Claimed    bool    `json:"claimed,omitempty"`
}

// Queue is the full queue file, including scan metadata.
type Queue struct {
	ScannedAt string `json:"scanned_at"`
	Items     []Item `json:"items"`
}

// Path returns the path to queue.json.
func Path() string {
	return home.Path("queue.json")
}

// Load reads queue items. Supports both legacy flat array and envelope format.
func Load() ([]Item, error) {
	q, err := LoadQueue()
	if err != nil {
		return nil, err
	}
	return q.Items, nil
}

// LoadQueue reads and parses queue.json returning the full envelope.
func LoadQueue() (Queue, error) {
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return Queue{}, nil
	}
	if err != nil {
		return Queue{}, err
	}
	// Support legacy flat array format.
	if len(data) > 0 && data[0] == '[' {
		var items []Item
		if err := json.Unmarshal(data, &items); err != nil {
			return Queue{}, err
		}
		return Queue{Items: items}, nil
	}
	var q Queue
	return q, json.Unmarshal(data, &q)
}

// Save writes q to queue.json.
func Save(q Queue) error {
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(Path(), data, 0o644)
}

// Clear removes queue.json entirely.
func Clear() error {
	err := os.Remove(Path())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
