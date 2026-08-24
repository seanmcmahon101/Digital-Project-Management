package store

import (
	"database/sql"
	"errors"
	"strings"
)

type RaidItem struct {
	ID          int64
	ProjectID   int64
	Ref         string
	Kind        string
	Title       string
	Detail      string
	Owner       string
	Probability int
	Impact      int
	Mitigation  string
	Status      string
	DueDate     string
	CreatedAt   string
	UpdatedAt   string
	ProjectCode string
	ProjectName string
}

// Score is probability x impact for risks (1-25). For other kinds it is
// just the impact.
func (r RaidItem) Score() int {
	if r.Kind == "risk" {
		return r.Probability * r.Impact
	}
	return r.Impact
}

// Severity buckets the risk score for display: high / medium / low.
func (r RaidItem) Severity() string {
	s := r.Score()
	switch {
	case r.Kind == "risk" && s >= 12, r.Kind != "risk" && s >= 4:
		return "high"
	case r.Kind == "risk" && s >= 6, r.Kind != "risk" && s >= 3:
		return "medium"
	default:
		return "low"
	}
}

var RaidKinds = []string{"risk", "issue", "assumption", "dependency"}

var RaidKindNames = map[string]string{
	"risk": "Risk", "issue": "Issue", "assumption": "Assumption", "dependency": "Dependency",
}

var raidPrefixes = map[string]string{
	"risk": "RISK", "issue": "ISS", "assumption": "ASM", "dependency": "DEP",
}

const raidCols = `r.id, r.project_id, r.ref, r.kind, r.title, r.detail, r.owner,
	r.probability, r.impact, r.mitigation, r.status, r.due_date, r.created_at, r.updated_at`

func scanRaid(row interface{ Scan(...any) error }) (RaidItem, error) {
	var r RaidItem
	err := row.Scan(&r.ID, &r.ProjectID, &r.Ref, &r.Kind, &r.Title, &r.Detail, &r.Owner,
		&r.Probability, &r.Impact, &r.Mitigation, &r.Status, &r.DueDate, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

func (s *Store) RaidItem(id int64) (RaidItem, error) {
	r, err := scanRaid(s.DB.QueryRow(`SELECT `+raidCols+` FROM raid_items r WHERE r.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// RaidItems returns a project's register, open items first, riskiest first.
func (s *Store) RaidItems(projectID int64) ([]RaidItem, error) {
	rows, err := s.DB.Query(`SELECT `+raidCols+` FROM raid_items r WHERE r.project_id = ?
		ORDER BY CASE r.status WHEN 'open' THEN 0 ELSE 1 END,
		(r.probability * r.impact) DESC, r.impact DESC, r.id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RaidItem
	for rows.Next() {
		r, err := scanRaid(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// OpenRisksAllProjects returns open risks and issues across active projects,
// highest score first.
func (s *Store) OpenRisksAllProjects() ([]RaidItem, error) {
	rows, err := s.DB.Query(`SELECT ` + raidCols + `, p.code, p.name
		FROM raid_items r JOIN projects p ON p.id = r.project_id
		WHERE r.status = 'open' AND r.kind IN ('risk','issue') AND p.status = 'active'
		ORDER BY (r.probability * r.impact) DESC, r.impact DESC, r.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RaidItem
	for rows.Next() {
		var r RaidItem
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Ref, &r.Kind, &r.Title, &r.Detail, &r.Owner,
			&r.Probability, &r.Impact, &r.Mitigation, &r.Status, &r.DueDate, &r.CreatedAt,
			&r.UpdatedAt, &r.ProjectCode, &r.ProjectName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateRaidItem(projectID int64, kind, title, detail, owner, mitigation, dueDate string, probability, impact int) (RaidItem, error) {
	prefix, ok := raidPrefixes[kind]
	if !ok {
		return RaidItem{}, &ValidationError{Problems: []string{"Invalid RAID type"}}
	}
	if err := Validate(Require(title, "Title"), ValidDate(dueDate, "Due date")); err != nil {
		return RaidItem{}, err
	}
	probability, impact = clampScore(probability), clampScore(impact)
	ref, err := s.NextRef(projectID, prefix)
	if err != nil {
		return RaidItem{}, err
	}
	res, err := s.DB.Exec(`INSERT INTO raid_items
		(project_id, ref, kind, title, detail, owner, probability, impact, mitigation, due_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		projectID, ref, kind, title, detail, owner, probability, impact, mitigation, dueDate)
	if err != nil {
		return RaidItem{}, err
	}
	id, _ := res.LastInsertId()
	s.LogActivity(projectID, kind, ref, "created", title)
	return s.RaidItem(id)
}

func clampScore(v int) int {
	if v < 0 {
		return 0
	}
	if v > 5 {
		return 5
	}
	return v
}

func (s *Store) UpdateRaidItem(id int64, title, detail, owner, mitigation, dueDate string, probability, impact int) error {
	if err := Validate(Require(title, "Title"), ValidDate(dueDate, "Due date")); err != nil {
		return err
	}
	_, err := s.DB.Exec(`UPDATE raid_items SET title=?, detail=?, owner=?, mitigation=?, due_date=?,
		probability=?, impact=?, updated_at = datetime('now') WHERE id = ?`,
		title, detail, owner, mitigation, dueDate, clampScore(probability), clampScore(impact), id)
	return err
}

func (s *Store) SetRaidStatus(id int64, status string) error {
	if status != "open" && status != "closed" {
		return &ValidationError{Problems: []string{"Invalid status"}}
	}
	_, err := s.DB.Exec(`UPDATE raid_items SET status=?, updated_at = datetime('now') WHERE id = ?`, status, id)
	if err != nil {
		return err
	}
	if r, rerr := s.RaidItem(id); rerr == nil {
		s.LogActivity(r.ProjectID, r.Kind, r.Ref, strings.TrimSuffix(status, "d")+"d", r.Title)
	}
	return nil
}

func (s *Store) DeleteRaidItem(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM raid_items WHERE id = ?`, id)
	return err
}
