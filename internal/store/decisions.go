package store

import (
	"database/sql"
	"errors"
)

type Decision struct {
	ID          int64
	ProjectID   int64
	Ref         string
	Title       string
	Context     string
	Outcome     string
	DecidedBy   string
	Status      string
	DecidedAt   string
	CreatedAt   string
	ProjectCode string
	ProjectName string
}

const decisionCols = `d.id, d.project_id, d.ref, d.title, d.context, d.outcome,
	d.decided_by, d.status, d.decided_at, d.created_at`

func scanDecision(row interface{ Scan(...any) error }) (Decision, error) {
	var d Decision
	err := row.Scan(&d.ID, &d.ProjectID, &d.Ref, &d.Title, &d.Context, &d.Outcome,
		&d.DecidedBy, &d.Status, &d.DecidedAt, &d.CreatedAt)
	return d, err
}

func (s *Store) Decision(id int64) (Decision, error) {
	d, err := scanDecision(s.DB.QueryRow(`SELECT `+decisionCols+` FROM decisions d WHERE d.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

func (s *Store) Decisions(projectID int64) ([]Decision, error) {
	rows, err := s.DB.Query(`SELECT `+decisionCols+` FROM decisions d WHERE d.project_id = ?
		ORDER BY CASE d.status WHEN 'pending' THEN 0 ELSE 1 END, d.id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Decision
	for rows.Next() {
		d, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DecisionsAllProjects returns decisions across all projects, pending first.
func (s *Store) DecisionsAllProjects() ([]Decision, error) {
	rows, err := s.DB.Query(`SELECT ` + decisionCols + `, p.code, p.name
		FROM decisions d JOIN projects p ON p.id = d.project_id
		ORDER BY CASE d.status WHEN 'pending' THEN 0 ELSE 1 END, d.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Decision
	for rows.Next() {
		var d Decision
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Ref, &d.Title, &d.Context, &d.Outcome,
			&d.DecidedBy, &d.Status, &d.DecidedAt, &d.CreatedAt, &d.ProjectCode, &d.ProjectName); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) CreateDecision(projectID int64, title, context string) (Decision, error) {
	if err := Validate(Require(title, "Decision title")); err != nil {
		return Decision{}, err
	}
	ref, err := s.NextRef(projectID, "DEC")
	if err != nil {
		return Decision{}, err
	}
	res, err := s.DB.Exec(`INSERT INTO decisions (project_id, ref, title, context)
		VALUES (?, ?, ?, ?)`, projectID, ref, title, context)
	if err != nil {
		return Decision{}, err
	}
	id, _ := res.LastInsertId()
	s.LogActivity(projectID, "decision", ref, "raised", title)
	return s.Decision(id)
}

// RecordDecision captures the outcome and marks the decision made.
func (s *Store) RecordDecision(id int64, outcome, decidedBy string) error {
	if err := Validate(Require(outcome, "Decision outcome")); err != nil {
		return err
	}
	_, err := s.DB.Exec(`UPDATE decisions SET outcome=?, decided_by=?, status='decided',
		decided_at=? WHERE id = ?`, outcome, decidedBy, Today(), id)
	if err != nil {
		return err
	}
	if d, derr := s.Decision(id); derr == nil {
		s.LogActivity(d.ProjectID, "decision", d.Ref, "decided", outcome)
	}
	return nil
}

func (s *Store) UpdateDecision(id int64, title, context, outcome, decidedBy string) error {
	if err := Validate(Require(title, "Decision title")); err != nil {
		return err
	}
	_, err := s.DB.Exec(`UPDATE decisions SET title=?, context=?, outcome=?, decided_by=?
		WHERE id = ?`, title, context, outcome, decidedBy, id)
	return err
}

func (s *Store) DeleteDecision(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM decisions WHERE id = ?`, id)
	return err
}
