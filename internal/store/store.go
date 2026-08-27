// Package store provides typed access to the application database.
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Store wraps the database with typed queries for each domain area.
type Store struct {
	DB *sql.DB
}

func New(db *sql.DB) *Store { return &Store{DB: db} }

// Today returns the current local date as YYYY-MM-DD.
func Today() string { return time.Now().Format("2006-01-02") }

// NextRef allocates the next readable reference for a project and kind,
// e.g. NextRef(3, "REQ") -> "REQ-001". projectID 0 is the global scope
// used for project codes.
func (s *Store) NextRef(projectID int64, prefix string) (string, error) {
	var next int64
	err := s.DB.QueryRow(`
		INSERT INTO ref_counters (project_id, kind, next) VALUES (?, ?, 2)
		ON CONFLICT (project_id, kind) DO UPDATE SET next = next + 1
		RETURNING next`, projectID, prefix).Scan(&next)
	if err != nil {
		return "", fmt.Errorf("allocate ref %s: %w", prefix, err)
	}
	return fmt.Sprintf("%s-%03d", prefix, next-1), nil
}

// Setting returns a setting value, or fallback if unset.
func (s *Store) Setting(key, fallback string) string {
	var v string
	err := s.DB.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return fallback
	}
	return v
}

// SettingFloat returns a numeric setting, or fallback if unset/invalid.
func (s *Store) SettingFloat(key string, fallback float64) float64 {
	var v float64
	err := s.DB.QueryRow(`SELECT CAST(value AS REAL) FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil || v == 0 {
		return fallback
	}
	return v
}

// SetSetting inserts or updates a setting.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.DB.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// SetSettings saves a related group of preferences atomically.
func (s *Store) SetSettings(settings map[string]string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, value := range settings {
		if _, err := tx.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LogActivity appends an entry to a project's activity history. Logging
// failures are swallowed: history must never block the action itself.
func (s *Store) LogActivity(projectID int64, entity, entityRef, action, detail string) {
	s.DB.Exec(`INSERT INTO activity_log (project_id, entity, entity_ref, action, detail)
		VALUES (?, ?, ?, ?, ?)`, projectID, entity, entityRef, action, detail)
}

// ActivityEntry is one row of a project's history.
type ActivityEntry struct {
	ID        int64
	ProjectID int64
	Entity    string
	EntityRef string
	Action    string
	Detail    string
	CreatedAt string
}

// Activity returns the most recent history entries for a project.
func (s *Store) Activity(projectID int64, limit int) ([]ActivityEntry, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, entity, entity_ref, action, detail, created_at
		FROM activity_log WHERE project_id = ? ORDER BY id DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActivityEntry
	for rows.Next() {
		var a ActivityEntry
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Entity, &a.EntityRef, &a.Action, &a.Detail, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ValidationError describes user-facing input problems.
type ValidationError struct{ Problems []string }

func (e *ValidationError) Error() string { return strings.Join(e.Problems, "; ") }

// Validate returns a ValidationError if any problem strings are non-empty.
func Validate(problems ...string) error {
	var real []string
	for _, p := range problems {
		if p != "" {
			real = append(real, p)
		}
	}
	if len(real) == 0 {
		return nil
	}
	return &ValidationError{Problems: real}
}

// Require returns a problem string when a required field is blank.
func Require(value, label string) string {
	if strings.TrimSpace(value) == "" {
		return label + " is required"
	}
	return ""
}

// ValidDate returns a problem string when value is neither blank nor a
// valid YYYY-MM-DD date.
func ValidDate(value, label string) string {
	if value == "" {
		return ""
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return label + " must be a date in YYYY-MM-DD format"
	}
	return ""
}

// DaysUntil returns the whole days from today until an ISO date. Negative
// means the date is in the past. ok is false when the date is blank/invalid.
func DaysUntil(date string) (days int, ok bool) {
	if date == "" {
		return 0, false
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0, false
	}
	now, _ := time.Parse("2006-01-02", Today())
	return int(t.Sub(now).Hours() / 24), true
}
