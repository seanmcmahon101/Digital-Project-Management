package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Stage identifiers in lifecycle order.
var Stages = []string{"intake", "discovery", "define", "plan", "build", "implement", "benefits"}

// StageNames maps stage ids to display names.
var StageNames = map[string]string{
	"intake":    "Intake",
	"discovery": "Discovery",
	"define":    "Define",
	"plan":      "Plan",
	"build":     "Build & Test",
	"implement": "Implement",
	"benefits":  "Benefits & Close",
}

// StageIndex returns the position of a stage in the lifecycle (0-based),
// or -1 if unknown.
func StageIndex(stage string) int {
	for i, s := range Stages {
		if s == stage {
			return i
		}
	}
	return -1
}

// NextStage returns the stage after the given one, or "" at the end.
func NextStage(stage string) string {
	i := StageIndex(stage)
	if i < 0 || i >= len(Stages)-1 {
		return ""
	}
	return Stages[i+1]
}

type Project struct {
	ID               int64
	Code             string
	Name             string
	Stage            string
	Status           string
	Sponsor          string
	Lead             string
	Department       string
	ProblemStatement string
	Goal             string
	CurrentState     string
	BusinessCase     string
	ScopeIn          string
	ScopeOut         string
	StartDate        string
	TargetEnd        string
	GoLive           string
	ClosedAt         string
	ClosureSummary   string
	CreatedAt        string
	UpdatedAt        string
}

func (p Project) StageName() string { return StageNames[p.Stage] }
func (p Project) IsClosed() bool    { return p.Status == "closed" || p.Status == "cancelled" }

const projectCols = `id, code, name, stage, status, sponsor, lead, department,
	problem_statement, goal, current_state, business_case, scope_in, scope_out,
	start_date, target_end, go_live, closed_at, closure_summary, created_at, updated_at`

func scanProject(row interface{ Scan(...any) error }) (Project, error) {
	var p Project
	err := row.Scan(&p.ID, &p.Code, &p.Name, &p.Stage, &p.Status, &p.Sponsor, &p.Lead,
		&p.Department, &p.ProblemStatement, &p.Goal, &p.CurrentState, &p.BusinessCase,
		&p.ScopeIn, &p.ScopeOut, &p.StartDate, &p.TargetEnd, &p.GoLive, &p.ClosedAt,
		&p.ClosureSummary, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

var ErrNotFound = errors.New("not found")

func (s *Store) Project(id int64) (Project, error) {
	p, err := scanProject(s.DB.QueryRow(`SELECT `+projectCols+` FROM projects WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// Projects returns all projects, active first, then by code.
func (s *Store) Projects() ([]Project, error) {
	rows, err := s.DB.Query(`SELECT ` + projectCols + ` FROM projects
		ORDER BY CASE status WHEN 'active' THEN 0 WHEN 'on_hold' THEN 1 ELSE 2 END, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateProject inserts a new project in Intake with a generated DPM code.
func (s *Store) CreateProject(name, sponsor, lead, department, problem, goal string) (Project, error) {
	if err := Validate(Require(name, "Project name")); err != nil {
		return Project{}, err
	}
	code, err := s.NextRef(0, "DPM")
	if err != nil {
		return Project{}, err
	}
	res, err := s.DB.Exec(`INSERT INTO projects
		(code, name, sponsor, lead, department, problem_statement, goal, start_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		code, strings.TrimSpace(name), sponsor, lead, department, problem, goal, Today())
	if err != nil {
		return Project{}, fmt.Errorf("create project: %w", err)
	}
	id, _ := res.LastInsertId()
	s.LogActivity(id, "project", code, "created", "Project created in Intake")
	return s.Project(id)
}

// UpdateProjectFields updates a whitelisted set of text columns on a project.
// fields maps column name -> new value.
func (s *Store) UpdateProjectFields(id int64, fields map[string]string) error {
	allowed := map[string]bool{
		"name": true, "sponsor": true, "lead": true, "department": true,
		"problem_statement": true, "goal": true, "current_state": true,
		"business_case": true, "scope_in": true, "scope_out": true,
		"start_date": true, "target_end": true, "go_live": true,
		"closure_summary": true, "status": true,
	}
	var sets []string
	var args []any
	for col, val := range fields {
		if !allowed[col] {
			return fmt.Errorf("field %q may not be updated", col)
		}
		sets = append(sets, col+" = ?")
		args = append(args, val)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = datetime('now')")
	args = append(args, id)
	_, err := s.DB.Exec(`UPDATE projects SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	return err
}

// AdvanceStage moves a project to the next stage, recording the gate
// transition. unmet lists criteria that were not satisfied; when non-empty
// the move is recorded as an override with the supplied reason.
func (s *Store) AdvanceStage(id int64, unmet []string, overrideReason string) (Project, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return Project{}, err
	}
	defer tx.Rollback()
	p, err := scanProject(tx.QueryRow(`SELECT `+projectCols+` FROM projects WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	if p.Status != "active" {
		return p, &ValidationError{Problems: []string{"Resume the project before advancing its stage"}}
	}
	next := NextStage(p.Stage)
	if next == "" {
		return p, errors.New("project is already at the final stage")
	}
	overridden := 0
	if len(unmet) > 0 {
		if strings.TrimSpace(overrideReason) == "" {
			return p, &ValidationError{Problems: []string{"An override reason is required when gate criteria are not met"}}
		}
		overridden = 1
	}
	_, err = tx.Exec(`UPDATE projects SET stage = ?, updated_at = datetime('now') WHERE id = ?`, next, id)
	if err != nil {
		return p, err
	}
	_, err = tx.Exec(`INSERT INTO gate_history (project_id, from_stage, to_stage, overridden, override_reason, unmet_criteria)
		VALUES (?, ?, ?, ?, ?, ?)`, id, p.Stage, next, overridden, overrideReason, strings.Join(unmet, "\n"))
	if err != nil {
		return p, err
	}
	detail := fmt.Sprintf("Moved from %s to %s", StageNames[p.Stage], StageNames[next])
	if overridden == 1 {
		detail += " (gate overridden: " + overrideReason + ")"
	}
	if _, err = tx.Exec(`INSERT INTO activity_log (project_id, entity, entity_ref, action, detail)
		VALUES (?, 'gate', '', 'stage_advanced', ?)`, id, detail); err != nil {
		return p, err
	}
	if err = tx.Commit(); err != nil {
		return p, err
	}
	return s.Project(id)
}

// CloseProject marks a project closed with a closure summary.
func (s *Store) CloseProject(id int64, summary string, cancelled bool) error {
	status := "closed"
	if cancelled {
		status = "cancelled"
	}
	_, err := s.DB.Exec(`UPDATE projects SET status = ?, closed_at = ?, closure_summary = ?,
		updated_at = datetime('now') WHERE id = ?`, status, Today(), summary, id)
	if err != nil {
		return err
	}
	s.LogActivity(id, "project", "", status, summary)
	return nil
}

// ReopenProject returns a closed project to active.
func (s *Store) ReopenProject(id int64) error {
	_, err := s.DB.Exec(`UPDATE projects SET status = 'active', closed_at = '',
		updated_at = datetime('now') WHERE id = ?`, id)
	if err != nil {
		return err
	}
	s.LogActivity(id, "project", "", "reopened", "Project reopened")
	return nil
}

// SetProjectHold pauses or resumes an active project and records the reason in
// the activity history. Closed projects must be reopened through their
// dedicated workflow first.
func (s *Store) SetProjectHold(id int64, hold bool, reason string) error {
	p, err := s.Project(id)
	if err != nil {
		return err
	}
	if p.IsClosed() {
		return &ValidationError{Problems: []string{"Reopen the project before changing its hold status"}}
	}
	reason = strings.TrimSpace(reason)
	if hold && reason == "" {
		return &ValidationError{Problems: []string{"Add a reason for putting the project on hold"}}
	}
	status, action, detail := "on_hold", "put_on_hold", reason
	if !hold {
		status, action, detail = "active", "resumed", "Project resumed"
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`, status, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO activity_log (project_id, entity, entity_ref, action, detail)
		VALUES (?, 'project', ?, ?, ?)`, id, p.Code, action, detail); err != nil {
		return err
	}
	return tx.Commit()
}

// GateEntry is one recorded stage transition.
type GateEntry struct {
	ID             int64
	ProjectID      int64
	FromStage      string
	ToStage        string
	Overridden     bool
	OverrideReason string
	UnmetCriteria  string
	MovedAt        string
}

func (g GateEntry) FromName() string { return StageNames[g.FromStage] }
func (g GateEntry) ToName() string   { return StageNames[g.ToStage] }

func (s *Store) GateHistory(projectID int64) ([]GateEntry, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, from_stage, to_stage, overridden,
		override_reason, unmet_criteria, moved_at FROM gate_history
		WHERE project_id = ? ORDER BY id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GateEntry
	for rows.Next() {
		var g GateEntry
		var ov int
		if err := rows.Scan(&g.ID, &g.ProjectID, &g.FromStage, &g.ToStage, &ov,
			&g.OverrideReason, &g.UnmetCriteria, &g.MovedAt); err != nil {
			return nil, err
		}
		g.Overridden = ov == 1
		out = append(out, g)
	}
	return out, rows.Err()
}

// DeleteProject permanently removes a project and all its child records.
func (s *Store) DeleteProject(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM projects WHERE id = ?`, id)
	return err
}
