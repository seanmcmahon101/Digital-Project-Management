package store

import (
	"database/sql"
	"errors"
	"strings"
)

type Requirement struct {
	ID        int64
	ProjectID int64
	Ref       string
	Title     string
	Detail    string
	Moscow    string
	Status    string
	Source    string
	CreatedAt string
	UpdatedAt string
	// Populated by Requirements(): tests linked to this requirement.
	Tests []Test
}

// TestSummary reports coverage for the requirement: "untested",
// "failing", "passing" or "in_progress".
func (r Requirement) TestSummary() string {
	if len(r.Tests) == 0 {
		return "untested"
	}
	allPass := true
	for _, t := range r.Tests {
		if t.Status == "fail" {
			return "failing"
		}
		if t.Status != "pass" {
			allPass = false
		}
	}
	if allPass {
		return "passing"
	}
	return "in_progress"
}

var MoscowNames = map[string]string{
	"must": "Must have", "should": "Should have", "could": "Could have", "wont": "Won't have",
}

func (s *Store) Requirement(id int64) (Requirement, error) {
	var r Requirement
	err := s.DB.QueryRow(`SELECT id, project_id, ref, title, detail, moscow, status, source, created_at, updated_at
		FROM requirements WHERE id = ?`, id).
		Scan(&r.ID, &r.ProjectID, &r.Ref, &r.Title, &r.Detail, &r.Moscow, &r.Status, &r.Source, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// Requirements returns a project's requirements with their linked tests,
// ordered by MoSCoW priority then ref.
func (s *Store) Requirements(projectID int64) ([]Requirement, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, ref, title, detail, moscow, status, source, created_at, updated_at
		FROM requirements WHERE project_id = ?
		ORDER BY CASE moscow WHEN 'must' THEN 0 WHEN 'should' THEN 1 WHEN 'could' THEN 2 ELSE 3 END, ref`, projectID)
	if err != nil {
		return nil, err
	}
	var out []Requirement
	index := map[int64]int{}
	for rows.Next() {
		var r Requirement
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Ref, &r.Title, &r.Detail, &r.Moscow,
			&r.Status, &r.Source, &r.CreatedAt, &r.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		index[r.ID] = len(out)
		out = append(out, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tests, err := s.Tests(projectID)
	if err != nil {
		return nil, err
	}
	for _, t := range tests {
		if t.RequirementID.Valid {
			if i, ok := index[t.RequirementID.Int64]; ok {
				out[i].Tests = append(out[i].Tests, t)
			}
		}
	}
	return out, nil
}

func (s *Store) CreateRequirement(projectID int64, title, detail, moscow, source string) (Requirement, error) {
	if err := Validate(Require(title, "Requirement title")); err != nil {
		return Requirement{}, err
	}
	if _, ok := MoscowNames[moscow]; !ok {
		moscow = "must"
	}
	ref, err := s.NextRef(projectID, "REQ")
	if err != nil {
		return Requirement{}, err
	}
	res, err := s.DB.Exec(`INSERT INTO requirements (project_id, ref, title, detail, moscow, source)
		VALUES (?, ?, ?, ?, ?, ?)`, projectID, ref, title, detail, moscow, source)
	if err != nil {
		return Requirement{}, err
	}
	id, _ := res.LastInsertId()
	s.LogActivity(projectID, "requirement", ref, "created", title)
	return s.Requirement(id)
}

func (s *Store) UpdateRequirement(id int64, title, detail, moscow, status, source string) error {
	if err := Validate(Require(title, "Requirement title")); err != nil {
		return err
	}
	if _, ok := MoscowNames[moscow]; !ok {
		moscow = "must"
	}
	switch status {
	case "proposed", "approved", "delivered", "dropped":
	default:
		status = "proposed"
	}
	_, err := s.DB.Exec(`UPDATE requirements SET title=?, detail=?, moscow=?, status=?, source=?,
		updated_at = datetime('now') WHERE id = ?`, title, detail, moscow, status, source, id)
	return err
}

func (s *Store) DeleteRequirement(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM requirements WHERE id = ?`, id)
	return err
}

// --- Tests ---

type Test struct {
	ID            int64
	ProjectID     int64
	Ref           string
	RequirementID sql.NullInt64
	Name          string
	Steps         string
	Expected      string
	Status        string
	TestedBy      string
	TestedAt      string
	Notes         string
	CreatedAt     string
	// Joined for display.
	RequirementRef string
}

var TestStatusNames = map[string]string{
	"not_run": "Not run", "pass": "Pass", "fail": "Fail", "blocked": "Blocked",
}

func (s *Store) Tests(projectID int64) ([]Test, error) {
	rows, err := s.DB.Query(`SELECT t.id, t.project_id, t.ref, t.requirement_id, t.name, t.steps,
		t.expected, t.status, t.tested_by, t.tested_at, t.notes, t.created_at,
		COALESCE(r.ref, '')
		FROM tests t LEFT JOIN requirements r ON r.id = t.requirement_id
		WHERE t.project_id = ? ORDER BY t.ref`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Test
	for rows.Next() {
		var t Test
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Ref, &t.RequirementID, &t.Name, &t.Steps,
			&t.Expected, &t.Status, &t.TestedBy, &t.TestedAt, &t.Notes, &t.CreatedAt,
			&t.RequirementRef); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CreateTest(projectID, requirementID int64, name, steps, expected string) error {
	if err := Validate(Require(name, "Test name")); err != nil {
		return err
	}
	ref, err := s.NextRef(projectID, "TEST")
	if err != nil {
		return err
	}
	var req any
	if requirementID > 0 {
		req = requirementID
	}
	_, err = s.DB.Exec(`INSERT INTO tests (project_id, ref, requirement_id, name, steps, expected)
		VALUES (?, ?, ?, ?, ?, ?)`, projectID, ref, req, name, steps, expected)
	if err == nil {
		s.LogActivity(projectID, "test", ref, "created", name)
	}
	return err
}

// SetTestStatus records a test run result.
func (s *Store) SetTestStatus(id int64, status, testedBy, notes string) error {
	if _, ok := TestStatusNames[status]; !ok {
		return &ValidationError{Problems: []string{"Invalid test status"}}
	}
	testedAt := Today()
	if status == "not_run" {
		testedAt = ""
	}
	_, err := s.DB.Exec(`UPDATE tests SET status=?, tested_by=?, tested_at=?,
		notes = CASE WHEN ? != '' THEN ? ELSE notes END WHERE id = ?`,
		status, testedBy, testedAt, notes, notes, id)
	if err != nil {
		return err
	}
	var projectID int64
	var ref, name string
	if s.DB.QueryRow(`SELECT project_id, ref, name FROM tests WHERE id = ?`, id).Scan(&projectID, &ref, &name) == nil {
		s.LogActivity(projectID, "test", ref, "result_"+status, name)
	}
	return nil
}

func (s *Store) UpdateTest(id, requirementID int64, name, steps, expected string) error {
	if err := Validate(Require(name, "Test name")); err != nil {
		return err
	}
	var req any
	if requirementID > 0 {
		req = requirementID
	}
	_, err := s.DB.Exec(`UPDATE tests SET requirement_id=?, name=?, steps=?, expected=? WHERE id = ?`,
		req, name, steps, expected, id)
	return err
}

func (s *Store) DeleteTest(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM tests WHERE id = ?`, id)
	return err
}

// --- Change requests ---

type ChangeRequest struct {
	ID                 int64
	ProjectID          int64
	Ref                string
	Title              string
	Description        string
	Impact             string
	RaisedBy           string
	Status             string
	DecidedAt          string
	CreatedAt          string
	CostImpact         float64
	ScheduleImpactDays int
	TargetDateImpact   string
	ScopeItems         []ScopeItem
	Requirements       []Requirement
}

func (c ChangeRequest) AffectsScope(id int64) bool {
	for _, item := range c.ScopeItems {
		if item.ID == id {
			return true
		}
	}
	return false
}

func (c ChangeRequest) AffectsRequirement(id int64) bool {
	for _, req := range c.Requirements {
		if req.ID == id {
			return true
		}
	}
	return false
}

func (s *Store) ChangeRequests(projectID int64) ([]ChangeRequest, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, ref, title, description, impact, raised_by,
		status, decided_at, created_at, cost_impact, schedule_impact_days, target_date_impact
		FROM change_requests WHERE project_id = ?
		ORDER BY CASE status WHEN 'proposed' THEN 0 ELSE 1 END, id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChangeRequest
	for rows.Next() {
		var c ChangeRequest
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Ref, &c.Title, &c.Description, &c.Impact,
			&c.RaisedBy, &c.Status, &c.DecidedAt, &c.CreatedAt, &c.CostImpact,
			&c.ScheduleImpactDays, &c.TargetDateImpact); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.loadChangeLinks(projectID, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) CreateChangeRequest(projectID int64, title, description, impact, raisedBy string) error {
	return s.CreateChangeRequestWithImpact(projectID, title, description, impact, raisedBy, 0, 0, "", nil, nil)
}

func (s *Store) ChangeRequest(id int64) (ChangeRequest, error) {
	var projectID int64
	if err := s.DB.QueryRow(`SELECT project_id FROM change_requests WHERE id = ?`, id).Scan(&projectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ChangeRequest{}, ErrNotFound
		}
		return ChangeRequest{}, err
	}
	items, err := s.ChangeRequests(projectID)
	if err != nil {
		return ChangeRequest{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return ChangeRequest{}, ErrNotFound
}

// CreateChangeRequestWithImpact records quantified impact and traceability to
// any affected scope items and requirements.
func (s *Store) CreateChangeRequestWithImpact(projectID int64, title, description, impact, raisedBy string,
	costImpact float64, scheduleImpactDays int, targetDateImpact string, scopeItemIDs, requirementIDs []int64) error {
	if err := Validate(Require(title, "Change request title")); err != nil {
		return err
	}
	if costImpact < 0 {
		return &ValidationError{Problems: []string{"Cost impact cannot be negative"}}
	}
	if problem := ValidDate(targetDateImpact, "Revised target date"); problem != "" {
		return &ValidationError{Problems: []string{problem}}
	}
	scopeItemIDs, requirementIDs, err := s.validateChangeLinks(projectID, scopeItemIDs, requirementIDs)
	if err != nil {
		return err
	}
	ref, err := s.NextRef(projectID, "CR")
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO change_requests (project_id, ref, title, description, impact,
		raised_by, cost_impact, schedule_impact_days, target_date_impact)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, projectID, ref, strings.TrimSpace(title),
		strings.TrimSpace(description), strings.TrimSpace(impact), strings.TrimSpace(raisedBy),
		costImpact, scheduleImpactDays, targetDateImpact)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertChangeLinks(tx, id, scopeItemIDs, requirementIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.LogActivity(projectID, "change_request", ref, "raised", title)
	return nil
}

func (s *Store) UpdateChangeRequest(id int64, title, description, impact, raisedBy string,
	costImpact float64, scheduleImpactDays int, targetDateImpact string, scopeItemIDs, requirementIDs []int64) error {
	c, err := s.ChangeRequest(id)
	if err != nil {
		return err
	}
	if c.Status != "proposed" {
		return &ValidationError{Problems: []string{"Only proposed change requests can be edited"}}
	}
	if err := Validate(Require(title, "Change request title")); err != nil {
		return err
	}
	if costImpact < 0 {
		return &ValidationError{Problems: []string{"Cost impact cannot be negative"}}
	}
	if problem := ValidDate(targetDateImpact, "Revised target date"); problem != "" {
		return &ValidationError{Problems: []string{problem}}
	}
	scopeItemIDs, requirementIDs, err = s.validateChangeLinks(c.ProjectID, scopeItemIDs, requirementIDs)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE change_requests SET title=?, description=?, impact=?, raised_by=?,
		cost_impact=?, schedule_impact_days=?, target_date_impact=? WHERE id=?`, strings.TrimSpace(title),
		strings.TrimSpace(description), strings.TrimSpace(impact), strings.TrimSpace(raisedBy), costImpact,
		scheduleImpactDays, targetDateImpact, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM change_scope_items WHERE change_request_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM change_requirements WHERE change_request_id=?`, id); err != nil {
		return err
	}
	if err := insertChangeLinks(tx, id, scopeItemIDs, requirementIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.LogActivity(c.ProjectID, "change_request", c.Ref, "updated", title)
	return nil
}

type sqlExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func insertChangeLinks(tx sqlExecer, changeID int64, scopeItemIDs, requirementIDs []int64) error {
	for _, itemID := range scopeItemIDs {
		if _, err := tx.Exec(`INSERT INTO change_scope_items (change_request_id, scope_item_id) VALUES (?, ?)`, changeID, itemID); err != nil {
			return err
		}
	}
	for _, reqID := range requirementIDs {
		if _, err := tx.Exec(`INSERT INTO change_requirements (change_request_id, requirement_id) VALUES (?, ?)`, changeID, reqID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) validateChangeLinks(projectID int64, scopeIDs, reqIDs []int64) ([]int64, []int64, error) {
	unique := func(ids []int64) []int64 {
		seen := map[int64]bool{}
		out := make([]int64, 0, len(ids))
		for _, id := range ids {
			if id > 0 && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
		return out
	}
	scopeIDs, reqIDs = unique(scopeIDs), unique(reqIDs)
	for _, id := range scopeIDs {
		var owner int64
		if err := s.DB.QueryRow(`SELECT project_id FROM scope_items WHERE id=?`, id).Scan(&owner); err != nil || owner != projectID {
			return nil, nil, &ValidationError{Problems: []string{"An affected scope item does not belong to this project"}}
		}
	}
	for _, id := range reqIDs {
		var owner int64
		if err := s.DB.QueryRow(`SELECT project_id FROM requirements WHERE id=?`, id).Scan(&owner); err != nil || owner != projectID {
			return nil, nil, &ValidationError{Problems: []string{"An affected requirement does not belong to this project"}}
		}
	}
	return scopeIDs, reqIDs, nil
}

func (s *Store) loadChangeLinks(projectID int64, changes []ChangeRequest) error {
	index := map[int64]int{}
	for i := range changes {
		index[changes[i].ID] = i
	}
	rows, err := s.DB.Query(`SELECT cs.change_request_id, si.id, si.project_id, si.ref,
		si.classification, si.title, si.owner, si.rationale, si.acceptance_criteria, si.status,
		si.created_at, si.updated_at FROM change_scope_items cs
		JOIN scope_items si ON si.id=cs.scope_item_id JOIN change_requests c ON c.id=cs.change_request_id
		WHERE c.project_id=? ORDER BY si.ref`, projectID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var changeID int64
		var item ScopeItem
		if err := rows.Scan(&changeID, &item.ID, &item.ProjectID, &item.Ref, &item.Classification,
			&item.Title, &item.Owner, &item.Rationale, &item.AcceptanceCriteria, &item.Status,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			rows.Close()
			return err
		}
		if i, ok := index[changeID]; ok {
			changes[i].ScopeItems = append(changes[i].ScopeItems, item)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	rows, err = s.DB.Query(`SELECT cr.change_request_id, r.id, r.project_id, r.ref, r.title, r.detail,
		r.moscow, r.status, r.source, r.created_at, r.updated_at FROM change_requirements cr
		JOIN requirements r ON r.id=cr.requirement_id JOIN change_requests c ON c.id=cr.change_request_id
		WHERE c.project_id=? ORDER BY r.ref`, projectID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var changeID int64
		var req Requirement
		if err := rows.Scan(&changeID, &req.ID, &req.ProjectID, &req.Ref, &req.Title, &req.Detail,
			&req.Moscow, &req.Status, &req.Source, &req.CreatedAt, &req.UpdatedAt); err != nil {
			return err
		}
		if i, ok := index[changeID]; ok {
			changes[i].Requirements = append(changes[i].Requirements, req)
		}
	}
	return rows.Err()
}

// DecideChangeRequest approves, rejects or withdraws a change request.
func (s *Store) DecideChangeRequest(id int64, status string) error {
	switch status {
	case "approved", "rejected", "withdrawn":
	default:
		return &ValidationError{Problems: []string{"Invalid change request decision"}}
	}
	_, err := s.DB.Exec(`UPDATE change_requests SET status=?, decided_at=? WHERE id = ?`,
		status, Today(), id)
	if err != nil {
		return err
	}
	var projectID int64
	var ref, title string
	if s.DB.QueryRow(`SELECT project_id, ref, title FROM change_requests WHERE id = ?`, id).
		Scan(&projectID, &ref, &title) == nil {
		s.LogActivity(projectID, "change_request", ref, status, title)
	}
	return nil
}

func (s *Store) DeleteChangeRequest(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM change_requests WHERE id = ?`, id)
	return err
}
