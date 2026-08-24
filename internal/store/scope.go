package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ScopeItem is one governed boundary of a project. Legacy free-text scope on
// Project remains supported; structured items add ownership and testability.
type ScopeItem struct {
	ID                 int64
	ProjectID          int64
	Ref                string
	Classification     string
	Title              string
	Owner              string
	Rationale          string
	AcceptanceCriteria string
	Status             string
	CreatedAt          string
	UpdatedAt          string
}

var ScopeStatusNames = map[string]string{
	"proposed": "Proposed", "agreed": "Agreed", "delivered": "Delivered", "removed": "Removed",
}

func validScopeClassification(v string) bool { return v == "in" || v == "out" }

func (s *Store) ScopeItem(id int64) (ScopeItem, error) {
	var item ScopeItem
	err := s.DB.QueryRow(`SELECT id, project_id, ref, classification, title, owner, rationale,
		acceptance_criteria, status, created_at, updated_at FROM scope_items WHERE id = ?`, id).
		Scan(&item.ID, &item.ProjectID, &item.Ref, &item.Classification, &item.Title, &item.Owner,
			&item.Rationale, &item.AcceptanceCriteria, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func (s *Store) ScopeItems(projectID int64) ([]ScopeItem, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, ref, classification, title, owner, rationale,
		acceptance_criteria, status, created_at, updated_at FROM scope_items WHERE project_id = ?
		ORDER BY CASE classification WHEN 'in' THEN 0 ELSE 1 END,
		CASE status WHEN 'agreed' THEN 0 WHEN 'proposed' THEN 1 WHEN 'delivered' THEN 2 ELSE 3 END, ref`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScopeItem
	for rows.Next() {
		var item ScopeItem
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Ref, &item.Classification, &item.Title,
			&item.Owner, &item.Rationale, &item.AcceptanceCriteria, &item.Status,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateScopeItem(projectID int64, classification, title, owner, rationale, acceptanceCriteria, status string) (ScopeItem, error) {
	title = strings.TrimSpace(title)
	if err := Validate(Require(title, "Scope item")); err != nil {
		return ScopeItem{}, err
	}
	if !validScopeClassification(classification) {
		return ScopeItem{}, &ValidationError{Problems: []string{"Classification must be in scope or out of scope"}}
	}
	if _, ok := ScopeStatusNames[status]; !ok {
		status = "proposed"
	}
	ref, err := s.NextRef(projectID, "SCP")
	if err != nil {
		return ScopeItem{}, err
	}
	res, err := s.DB.Exec(`INSERT INTO scope_items
		(project_id, ref, classification, title, owner, rationale, acceptance_criteria, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, projectID, ref, classification, title,
		strings.TrimSpace(owner), strings.TrimSpace(rationale), strings.TrimSpace(acceptanceCriteria), status)
	if err != nil {
		return ScopeItem{}, err
	}
	id, _ := res.LastInsertId()
	s.LogActivity(projectID, "scope_item", ref, "created", title)
	return s.ScopeItem(id)
}

func (s *Store) UpdateScopeItem(id int64, classification, title, owner, rationale, acceptanceCriteria, status string) error {
	item, err := s.ScopeItem(id)
	if err != nil {
		return err
	}
	title = strings.TrimSpace(title)
	if err := Validate(Require(title, "Scope item")); err != nil {
		return err
	}
	if !validScopeClassification(classification) {
		return &ValidationError{Problems: []string{"Classification must be in scope or out of scope"}}
	}
	if _, ok := ScopeStatusNames[status]; !ok {
		return &ValidationError{Problems: []string{"Invalid scope status"}}
	}
	_, err = s.DB.Exec(`UPDATE scope_items SET classification=?, title=?, owner=?, rationale=?,
		acceptance_criteria=?, status=?, updated_at=datetime('now') WHERE id=?`, classification,
		title, strings.TrimSpace(owner), strings.TrimSpace(rationale), strings.TrimSpace(acceptanceCriteria), status, id)
	if err == nil {
		s.LogActivity(item.ProjectID, "scope_item", item.Ref, "updated", title+" ("+status+")")
	}
	return err
}

func (s *Store) DeleteScopeItem(id int64) error {
	item, err := s.ScopeItem(id)
	if err != nil {
		return err
	}
	var changeLinks int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM change_scope_items WHERE scope_item_id=?`, id).Scan(&changeLinks); err != nil {
		return err
	}
	if changeLinks > 0 {
		return &ValidationError{Problems: []string{"This scope item is linked to a change request; mark it Removed instead of deleting it"}}
	}
	res, err := s.DB.Exec(`DELETE FROM scope_items WHERE id = ?`, id)
	if err == nil {
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		s.LogActivity(item.ProjectID, "scope_item", item.Ref, "deleted", item.Title)
	}
	return err
}

type ScopeBaseline struct {
	ID             int64
	ProjectID      int64
	Version        int
	ApprovedBy     string
	ApprovedAt     string
	Notes          string
	ScopeSnapshot  string
	CreatedAt      string
	LegacyScopeIn  string
	LegacyScopeOut string
	Items          []ScopeItem
}

type scopeBaselineSnapshot struct {
	LegacyScopeIn  string      `json:"legacy_scope_in,omitempty"`
	LegacyScopeOut string      `json:"legacy_scope_out,omitempty"`
	Items          []ScopeItem `json:"items,omitempty"`
}

func (s *Store) ScopeBaselines(projectID int64) ([]ScopeBaseline, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, version, approved_by, approved_at, notes,
		scope_snapshot, created_at FROM scope_baselines WHERE project_id = ? ORDER BY version DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScopeBaseline
	for rows.Next() {
		var b ScopeBaseline
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.Version, &b.ApprovedBy, &b.ApprovedAt,
			&b.Notes, &b.ScopeSnapshot, &b.CreatedAt); err != nil {
			return nil, err
		}
		var snap scopeBaselineSnapshot
		if err := json.Unmarshal([]byte(b.ScopeSnapshot), &snap); err != nil {
			return nil, fmt.Errorf("decode scope baseline v%d: %w", b.Version, err)
		}
		b.LegacyScopeIn, b.LegacyScopeOut, b.Items = snap.LegacyScopeIn, snap.LegacyScopeOut, snap.Items
		out = append(out, b)
	}
	return out, rows.Err()
}

// ApproveScopeBaseline records an immutable copy of both the legacy scope text
// and structured items. Each approval creates a new monotonically increasing version.
func (s *Store) ApproveScopeBaseline(projectID int64, approvedBy, approvedAt, notes string) (ScopeBaseline, error) {
	approvedBy = strings.TrimSpace(approvedBy)
	if approvedAt == "" {
		approvedAt = Today()
	}
	if err := Validate(Require(approvedBy, "Approver"), ValidDate(approvedAt, "Approval date")); err != nil {
		return ScopeBaseline{}, err
	}
	p, err := s.Project(projectID)
	if err != nil {
		return ScopeBaseline{}, err
	}
	items, err := s.ScopeItems(projectID)
	if err != nil {
		return ScopeBaseline{}, err
	}
	if strings.TrimSpace(p.ScopeIn) == "" && strings.TrimSpace(p.ScopeOut) == "" && len(items) == 0 {
		return ScopeBaseline{}, &ValidationError{Problems: []string{"Add scope before approving a baseline"}}
	}
	snapshot, err := json.Marshal(scopeBaselineSnapshot{LegacyScopeIn: p.ScopeIn, LegacyScopeOut: p.ScopeOut, Items: items})
	if err != nil {
		return ScopeBaseline{}, fmt.Errorf("encode scope baseline: %w", err)
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return ScopeBaseline{}, err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(version), 0) + 1 FROM scope_baselines WHERE project_id = ?`, projectID).Scan(&version); err != nil {
		return ScopeBaseline{}, err
	}
	res, err := tx.Exec(`INSERT INTO scope_baselines
		(project_id, version, approved_by, approved_at, notes, scope_snapshot)
		VALUES (?, ?, ?, ?, ?, ?)`, projectID, version, approvedBy, approvedAt, strings.TrimSpace(notes), string(snapshot))
	if err != nil {
		return ScopeBaseline{}, err
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return ScopeBaseline{}, err
	}
	s.LogActivity(projectID, "scope_baseline", fmt.Sprintf("v%d", version), "approved", "Approved by "+approvedBy)
	return ScopeBaseline{ID: id, ProjectID: projectID, Version: version, ApprovedBy: approvedBy,
		ApprovedAt: approvedAt, Notes: strings.TrimSpace(notes), ScopeSnapshot: string(snapshot),
		LegacyScopeIn: p.ScopeIn, LegacyScopeOut: p.ScopeOut, Items: items}, nil
}
