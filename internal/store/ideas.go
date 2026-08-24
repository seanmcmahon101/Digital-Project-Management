package store

import (
	"database/sql"
	"errors"
)

type Idea struct {
	ID             int64
	Title          string
	Summary        string
	SubmittedBy    string
	Status         string
	ScoreValue     int
	ScoreUrgency   int
	ScoreAlignment int
	ScoreEffort    int
	ScoreRisk      int
	ProjectID      sql.NullInt64
	CreatedAt      string
	UpdatedAt      string
}

// Score is the weighted priority score: benefit-side scores count up,
// effort and risk count down. Range roughly -10 .. +40.
func (i Idea) Score() int {
	return i.ScoreValue*3 + i.ScoreUrgency*2 + i.ScoreAlignment*3 - i.ScoreEffort*2 - i.ScoreRisk
}

// Scored reports whether any scoring has been captured.
func (i Idea) Scored() bool {
	return i.ScoreValue+i.ScoreUrgency+i.ScoreAlignment+i.ScoreEffort+i.ScoreRisk > 0
}

const ideaCols = `id, title, summary, submitted_by, status, score_value, score_urgency,
	score_alignment, score_effort, score_risk, project_id, created_at, updated_at`

func scanIdea(row interface{ Scan(...any) error }) (Idea, error) {
	var i Idea
	err := row.Scan(&i.ID, &i.Title, &i.Summary, &i.SubmittedBy, &i.Status, &i.ScoreValue,
		&i.ScoreUrgency, &i.ScoreAlignment, &i.ScoreEffort, &i.ScoreRisk, &i.ProjectID,
		&i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (s *Store) Idea(id int64) (Idea, error) {
	i, err := scanIdea(s.DB.QueryRow(`SELECT `+ideaCols+` FROM ideas WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return i, ErrNotFound
	}
	return i, err
}

// Ideas returns all ideas ordered: open pipeline first by score descending,
// then parked/rejected/converted.
func (s *Store) Ideas() ([]Idea, error) {
	rows, err := s.DB.Query(`SELECT ` + ideaCols + ` FROM ideas
		ORDER BY CASE status WHEN 'converted' THEN 2 WHEN 'rejected' THEN 2 WHEN 'parked' THEN 1 ELSE 0 END,
		(score_value*3 + score_urgency*2 + score_alignment*3 - score_effort*2 - score_risk) DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Idea
	for rows.Next() {
		i, err := scanIdea(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) CreateIdea(title, summary, submittedBy string) (int64, error) {
	if err := Validate(Require(title, "Idea title")); err != nil {
		return 0, err
	}
	res, err := s.DB.Exec(`INSERT INTO ideas (title, summary, submitted_by) VALUES (?, ?, ?)`,
		title, summary, submittedBy)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ScoreIdea saves scoring values (each 0-5) and marks the idea scored.
func (s *Store) ScoreIdea(id int64, value, urgency, alignment, effort, risk int) error {
	for _, v := range []int{value, urgency, alignment, effort, risk} {
		if v < 0 || v > 5 {
			return &ValidationError{Problems: []string{"Scores must be between 0 and 5"}}
		}
	}
	_, err := s.DB.Exec(`UPDATE ideas SET score_value=?, score_urgency=?, score_alignment=?,
		score_effort=?, score_risk=?, status = CASE WHEN status='new' THEN 'scored' ELSE status END,
		updated_at = datetime('now') WHERE id = ?`, value, urgency, alignment, effort, risk, id)
	return err
}

func (s *Store) UpdateIdea(id int64, title, summary, submittedBy string) error {
	if err := Validate(Require(title, "Idea title")); err != nil {
		return err
	}
	_, err := s.DB.Exec(`UPDATE ideas SET title=?, summary=?, submitted_by=?,
		updated_at = datetime('now') WHERE id = ?`, title, summary, submittedBy, id)
	return err
}

func (s *Store) SetIdeaStatus(id int64, status string) error {
	switch status {
	case "new", "scored", "approved", "parked", "rejected":
	default:
		return &ValidationError{Problems: []string{"Invalid idea status"}}
	}
	_, err := s.DB.Exec(`UPDATE ideas SET status=?, updated_at = datetime('now') WHERE id = ?`, status, id)
	return err
}

// ConvertIdea creates a project from an idea and links the two.
func (s *Store) ConvertIdea(id int64, sponsor, lead string) (Project, error) {
	idea, err := s.Idea(id)
	if err != nil {
		return Project{}, err
	}
	if idea.Status == "converted" {
		return Project{}, &ValidationError{Problems: []string{"This idea has already been converted to a project"}}
	}
	p, err := s.CreateProject(idea.Title, sponsor, lead, "", idea.Summary, "")
	if err != nil {
		return Project{}, err
	}
	_, err = s.DB.Exec(`UPDATE ideas SET status='converted', project_id=?,
		updated_at = datetime('now') WHERE id = ?`, p.ID, id)
	return p, err
}

func (s *Store) DeleteIdea(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM ideas WHERE id = ?`, id)
	return err
}
