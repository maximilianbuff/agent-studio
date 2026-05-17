// Package db manages the AgentStudio SQLite database at ~/.agent-studio/studio.db.
package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/maximilianbuff/agent-studio/internal/home"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS items (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	repo       TEXT    NOT NULL,
	number     INTEGER NOT NULL,
	type       TEXT,
	title      TEXT,
	score      INTEGER DEFAULT 0,
	first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(repo, number)
);

CREATE TABLE IF NOT EXISTS events (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	item_id INTEGER NOT NULL REFERENCES items(id),
	status  TEXT    NOT NULL,
	at      DATETIME DEFAULT CURRENT_TIMESTAMP,
	pr_url  TEXT
);

CREATE TABLE IF NOT EXISTS prs (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	repo            TEXT    NOT NULL,
	number          INTEGER NOT NULL,
	author          TEXT    NOT NULL DEFAULT '',
	title           TEXT    NOT NULL DEFAULT '',
	url             TEXT    NOT NULL DEFAULT '',
	head_ref        TEXT    NOT NULL DEFAULT '',
	state           TEXT    NOT NULL DEFAULT '',
	mergeable       TEXT    NOT NULL DEFAULT '',
	review_decision TEXT    NOT NULL DEFAULT '',
	ci_status       TEXT    NOT NULL DEFAULT '',
	merged_at       TEXT    NOT NULL DEFAULT '',
	last_seen       DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(repo, number)
);

CREATE INDEX IF NOT EXISTS events_item_id ON events(item_id);
CREATE INDEX IF NOT EXISTS events_status   ON events(status);
CREATE INDEX IF NOT EXISTS prs_author      ON prs(author);
`

// DB wraps a SQLite connection.
type DB struct {
	db *sql.DB
}

// Path returns the path to studio.db.
func Path() string {
	return home.Path("studio.db")
}

// migrations are run after the base schema. Each is best-effort: "duplicate column"
// errors are silently ignored so they're safe to re-run on existing DBs.
var migrations = []string{
	`ALTER TABLE prs ADD COLUMN comment_count INTEGER NOT NULL DEFAULT 0`,
}

// Open opens (or creates) the database and applies the schema.
func Open() (*DB, error) {
	d, err := sql.Open("sqlite", Path()+"?_journal=WAL&_timeout=5000")
	if err != nil {
		return nil, err
	}
	if _, err := d.Exec(schema); err != nil {
		d.Close()
		return nil, fmt.Errorf("applying schema: %w", err)
	}
	for _, m := range migrations {
		if _, err := d.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			d.Close()
			return nil, fmt.Errorf("migration %q: %w", m, err)
		}
	}
	return &DB{db: d}, nil
}

// Close closes the underlying connection.
func (d *DB) Close() error { return d.db.Close() }

// UpsertItem inserts or updates a work item, returning its row ID.
func (d *DB) UpsertItem(repo string, number int, itemType, title string, score int) (int64, error) {
	_, err := d.db.Exec(`
		INSERT INTO items (repo, number, type, title, score)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(repo, number) DO UPDATE SET
			type  = excluded.type,
			title = excluded.title,
			score = excluded.score
	`, repo, number, nullStr(itemType), title, score)
	if err != nil {
		return 0, err
	}
	var id int64
	err = d.db.QueryRow(`SELECT id FROM items WHERE repo=? AND number=?`, repo, number).Scan(&id)
	return id, err
}

// AddEvent records a status transition for an item by its row ID.
func (d *DB) AddEvent(itemID int64, status, prURL string) error {
	_, err := d.db.Exec(`INSERT INTO events (item_id, status, pr_url) VALUES (?, ?, ?)`,
		itemID, status, nullStr(prURL))
	return err
}

// AddEventByRef records a status transition using repo+number.
// If the item is not yet in the DB it is created with minimal data.
func (d *DB) AddEventByRef(repo string, number int, status, prURL string) error {
	var id int64
	err := d.db.QueryRow(`SELECT id FROM items WHERE repo=? AND number=?`, repo, number).Scan(&id)
	if err == sql.ErrNoRows {
		id, err = d.UpsertItem(repo, number, "", "", 0)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return d.AddEvent(id, status, prURL)
}

// Item is a work item with its latest event status.
type Item struct {
	ID        int64
	Repo      string
	Number    int
	Type      string
	Title     string
	Score     int
	FirstSeen string
	Status    string
	PRUrl     string
	UpdatedAt string
}

// ListItems returns recent work items with their latest status, newest first.
func (d *DB) ListItems(limit int) ([]Item, error) {
	rows, err := d.db.Query(`
		SELECT i.id, i.repo, i.number, COALESCE(i.type,''), i.title, i.score, i.first_seen,
		       COALESCE(e.status,''), COALESCE(e.pr_url,''), COALESCE(e.at,'')
		FROM items i
		LEFT JOIN events e ON e.id = (
			SELECT id FROM events WHERE item_id = i.id ORDER BY at DESC LIMIT 1
		)
		ORDER BY COALESCE(e.at, i.first_seen) DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Repo, &it.Number, &it.Type, &it.Title,
			&it.Score, &it.FirstSeen, &it.Status, &it.PRUrl, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ListPendingIssues returns items whose latest event status is not pr_opened, merged, or closed.
// These are candidates for the worker queue.
func (d *DB) ListPendingIssues(limit int) ([]Item, error) {
	rows, err := d.db.Query(`
		SELECT i.id, i.repo, i.number, COALESCE(i.type,''), i.title, i.score, i.first_seen,
		       COALESCE(e.status,''), COALESCE(e.pr_url,'')
		FROM items i
		LEFT JOIN events e ON e.id = (
			SELECT id FROM events WHERE item_id = i.id ORDER BY at DESC LIMIT 1
		)
		WHERE COALESCE(e.status,'') NOT IN ('pr_opened','merged','closed')
		ORDER BY i.score DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Repo, &it.Number, &it.Type, &it.Title,
			&it.Score, &it.FirstSeen, &it.Status, &it.PRUrl); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// PRRecord holds the current state of a pull request as observed by the scanner.
type PRRecord struct {
	Repo           string
	Number         int
	Author         string
	Title          string
	URL            string
	HeadRef        string
	State          string // OPEN, CLOSED, MERGED
	Mergeable      string // MERGEABLE, CONFLICTING, UNKNOWN
	ReviewDecision string // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, ""
	CIStatus       string // SUCCESS, FAILURE, ERROR, PENDING, ""
	MergedAt       string
	CommentCount   int
}

// UpsertPR inserts or updates a PR record. Called by the scanner.
func (d *DB) UpsertPR(pr PRRecord) error {
	_, err := d.db.Exec(`
		INSERT INTO prs (repo, number, author, title, url, head_ref, state, mergeable,
		                 review_decision, ci_status, merged_at, comment_count, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		ON CONFLICT(repo, number) DO UPDATE SET
			author          = excluded.author,
			title           = excluded.title,
			url             = excluded.url,
			head_ref        = excluded.head_ref,
			state           = excluded.state,
			mergeable       = excluded.mergeable,
			review_decision = excluded.review_decision,
			ci_status       = excluded.ci_status,
			merged_at       = excluded.merged_at,
			comment_count   = excluded.comment_count,
			last_seen       = excluded.last_seen
	`, pr.Repo, pr.Number, pr.Author, pr.Title, pr.URL, pr.HeadRef,
		pr.State, pr.Mergeable, pr.ReviewDecision, pr.CIStatus, pr.MergedAt, pr.CommentCount)
	return err
}

// ListOpenPRs returns all open PRs for the given repos, ordered by state severity then last seen.
// repos is a slice of "owner/repo" strings; passing nil returns all repos.
func (d *DB) ListOpenPRs(repos []string) ([]PRRecord, error) {
	query := `
		SELECT repo, number, author, title, url, head_ref, state,
		       mergeable, review_decision, ci_status, merged_at, comment_count
		FROM prs
		WHERE state = 'OPEN'`
	var args []any
	if len(repos) > 0 {
		placeholders := make([]string, len(repos))
		for i, r := range repos {
			placeholders[i] = "?"
			args = append(args, r)
		}
		query += " AND repo IN (" + strings.Join(placeholders, ",") + ")"
	}
	query += " ORDER BY last_seen DESC"

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prs []PRRecord
	for rows.Next() {
		var pr PRRecord
		if err := rows.Scan(&pr.Repo, &pr.Number, &pr.Author, &pr.Title, &pr.URL,
			&pr.HeadRef, &pr.State, &pr.Mergeable, &pr.ReviewDecision,
			&pr.CIStatus, &pr.MergedAt, &pr.CommentCount); err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}
	return prs, rows.Err()
}

// RepoStat holds per-repo aggregate statistics.
type RepoStat struct {
	Repo       string
	Queued     int
	Done       int
	Failed     int
	AvgMinutes float64
}

// RepoStats returns per-repo statistics. Success rate is informational only.
func (d *DB) RepoStats() ([]RepoStat, error) {
	rows, err := d.db.Query(`
		SELECT
			i.repo,
			COUNT(DISTINCT i.id)                                                                   AS queued,
			COUNT(DISTINCT CASE WHEN e.status IN ('pr_opened','merged') THEN i.id END)             AS done,
			COUNT(DISTINCT CASE WHEN e.status = 'failed'                THEN i.id END)             AS failed,
			AVG(CASE WHEN e.status IN ('pr_opened','merged') THEN
				(julianday(e.at) - julianday(i.first_seen)) * 1440
			END)                                                                                   AS avg_minutes
		FROM items i
		LEFT JOIN events e ON e.item_id = i.id
		GROUP BY i.repo
		ORDER BY done DESC, queued DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []RepoStat
	for rows.Next() {
		var s RepoStat
		var avg sql.NullFloat64
		if err := rows.Scan(&s.Repo, &s.Queued, &s.Done, &s.Failed, &avg); err != nil {
			return nil, err
		}
		if avg.Valid {
			s.AvgMinutes = avg.Float64
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// Event is a single status transition.
type Event struct {
	Status string
	At     string
	PRUrl  string
}

// GetItemHistory returns the item and all its events ordered by time.
func (d *DB) GetItemHistory(repo string, number int) (*Item, []Event, error) {
	var it Item
	err := d.db.QueryRow(
		`SELECT id, repo, number, COALESCE(type,''), title, score, first_seen
		 FROM items WHERE repo=? AND number=?`,
		repo, number,
	).Scan(&it.ID, &it.Repo, &it.Number, &it.Type, &it.Title, &it.Score, &it.FirstSeen)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("item %s#%d not found", repo, number)
	}
	if err != nil {
		return nil, nil, err
	}
	rows, err := d.db.Query(
		`SELECT status, at, COALESCE(pr_url,'') FROM events WHERE item_id=? ORDER BY at`,
		it.ID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Status, &e.At, &e.PRUrl); err != nil {
			return nil, nil, err
		}
		events = append(events, e)
	}
	return &it, events, rows.Err()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
