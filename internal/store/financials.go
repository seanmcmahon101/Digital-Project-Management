package store

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
)

// ProjectFinancials is the editable investment baseline for a project.
// Return/value figures are deliberately sourced from measurable benefits.
type ProjectFinancials struct {
	ProjectID      int64
	EstimatedCost  float64
	ApprovedBudget float64
	ActualCost     float64
	Notes          string
	UpdatedAt      string
}

// InvestmentCost returns the strongest available cost figure: actual spend,
// then approved budget, then the early estimate.
func (f ProjectFinancials) InvestmentCost() float64 {
	if f.ActualCost > 0 {
		return f.ActualCost
	}
	if f.ApprovedBudget > 0 {
		return f.ApprovedBudget
	}
	return f.EstimatedCost
}

// BudgetVariance returns actual minus approved budget. Positive is overspend.
func (f ProjectFinancials) BudgetVariance() float64 { return f.ActualCost - f.ApprovedBudget }

// FinancialSummary joins project costs to benefit-derived value.
type FinancialSummary struct {
	Financials          ProjectFinancials
	ExpectedAnnualValue float64
	RealisedAnnualValue float64
	Investment          float64
	ROI                 float64 // first-year return on investment, percent
	PaybackMonths       float64
	HasROI              bool
	HasPayback          bool
}

// SummariseFinancials calculates value, first-year ROI, and simple payback.
func SummariseFinancials(fin ProjectFinancials, benefits []Benefit) FinancialSummary {
	s := FinancialSummary{Financials: fin, Investment: fin.InvestmentCost()}
	for _, b := range benefits {
		s.ExpectedAnnualValue += b.AnnualValue
		s.RealisedAnnualValue += b.RealisedAnnualValue()
	}
	if s.Investment > 0 {
		s.ROI = (s.ExpectedAnnualValue - s.Investment) / s.Investment * 100
		s.HasROI = true
	}
	if s.Investment > 0 && s.ExpectedAnnualValue > 0 {
		s.PaybackMonths = s.Investment / s.ExpectedAnnualValue * 12
		s.HasPayback = true
	}
	return s
}

// Financials returns a project's investment figures. Projects with no saved
// financial baseline return a zero-valued record rather than ErrNotFound.
func (s *Store) Financials(projectID int64) (ProjectFinancials, error) {
	f := ProjectFinancials{ProjectID: projectID}
	err := s.DB.QueryRow(`SELECT project_id, estimated_cost, approved_budget,
		actual_cost, notes, updated_at FROM project_financials WHERE project_id = ?`, projectID).
		Scan(&f.ProjectID, &f.EstimatedCost, &f.ApprovedBudget, &f.ActualCost, &f.Notes, &f.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return f, nil
	}
	return f, err
}

// SaveFinancials creates or replaces the project's financial baseline.
func (s *Store) SaveFinancials(projectID int64, estimated, approved, actual float64, notes string) error {
	for label, value := range map[string]float64{
		"Estimated cost": estimated, "Approved budget": approved, "Actual cost": actual,
	} {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return &ValidationError{Problems: []string{label + " must be a non-negative number"}}
		}
	}
	if len(strings.TrimSpace(notes)) > 2000 {
		return &ValidationError{Problems: []string{"Financial notes must be 2,000 characters or fewer"}}
	}
	_, err := s.DB.Exec(`INSERT INTO project_financials
		(project_id, estimated_cost, approved_budget, actual_cost, notes)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET
			estimated_cost = excluded.estimated_cost,
			approved_budget = excluded.approved_budget,
			actual_cost = excluded.actual_cost,
			notes = excluded.notes,
			updated_at = datetime('now')`, projectID, estimated, approved, actual, strings.TrimSpace(notes))
	if err != nil {
		return fmt.Errorf("save project financials: %w", err)
	}
	s.LogActivity(projectID, "financials", "", "updated", "Project costs and budget updated")
	return nil
}

// ProjectStatusSnapshot is an immutable point-in-time project baseline.
type ProjectStatusSnapshot struct {
	ID                  int64
	ProjectID           int64
	Stage               string
	ProjectStatus       string
	HealthStatus        string
	HealthSummary       string
	TotalTasks          int
	DoneTasks           int
	OpenRaidItems       int
	OverdueTasks        int
	OverdueMilestones   int
	ExpectedAnnualValue float64
	RealisedAnnualValue float64
	EstimatedCost       float64
	ApprovedBudget      float64
	ActualCost          float64
	Note                string
	CapturedAt          string
	HasPrevious         bool
	TaskCompletionDelta float64
	OpenRaidDelta       int
	OverdueDelta        int
	RealisedValueDelta  float64
	HealthMovement      string
}

func (s ProjectStatusSnapshot) TaskCompletion() float64 {
	if s.TotalTasks == 0 {
		return 0
	}
	return float64(s.DoneTasks) / float64(s.TotalTasks) * 100
}

func (s ProjectStatusSnapshot) TotalOverdue() int {
	return s.OverdueTasks + s.OverdueMilestones
}

// CreateStatusSnapshot stores a pre-calculated point-in-time project status.
func (s *Store) CreateStatusSnapshot(in ProjectStatusSnapshot) (ProjectStatusSnapshot, error) {
	if in.ProjectID <= 0 {
		return ProjectStatusSnapshot{}, &ValidationError{Problems: []string{"Project is required"}}
	}
	if len(strings.TrimSpace(in.Note)) > 1000 {
		return ProjectStatusSnapshot{}, &ValidationError{Problems: []string{"Snapshot note must be 1,000 characters or fewer"}}
	}
	if in.HealthStatus != "green" && in.HealthStatus != "amber" && in.HealthStatus != "red" && in.HealthStatus != "closed" {
		return ProjectStatusSnapshot{}, &ValidationError{Problems: []string{"Snapshot health status is invalid"}}
	}
	res, err := s.DB.Exec(`INSERT INTO project_status_snapshots
		(project_id, stage, project_status, health_status, health_summary,
		total_tasks, done_tasks, open_raid_items, overdue_tasks, overdue_milestones,
		expected_annual_value, realised_annual_value, estimated_cost, approved_budget,
		actual_cost, note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.ProjectID, in.Stage, in.ProjectStatus, in.HealthStatus, in.HealthSummary,
		in.TotalTasks, in.DoneTasks, in.OpenRaidItems, in.OverdueTasks, in.OverdueMilestones,
		in.ExpectedAnnualValue, in.RealisedAnnualValue, in.EstimatedCost, in.ApprovedBudget,
		in.ActualCost, strings.TrimSpace(in.Note))
	if err != nil {
		return ProjectStatusSnapshot{}, fmt.Errorf("create status snapshot: %w", err)
	}
	id, _ := res.LastInsertId()
	s.LogActivity(in.ProjectID, "status_snapshot", fmt.Sprintf("SNAP-%03d", id), "captured", strings.TrimSpace(in.Note))
	return s.StatusSnapshot(id)
}

func scanStatusSnapshot(row interface{ Scan(...any) error }) (ProjectStatusSnapshot, error) {
	var s ProjectStatusSnapshot
	err := row.Scan(&s.ID, &s.ProjectID, &s.Stage, &s.ProjectStatus, &s.HealthStatus,
		&s.HealthSummary, &s.TotalTasks, &s.DoneTasks, &s.OpenRaidItems, &s.OverdueTasks,
		&s.OverdueMilestones, &s.ExpectedAnnualValue, &s.RealisedAnnualValue,
		&s.EstimatedCost, &s.ApprovedBudget, &s.ActualCost, &s.Note, &s.CapturedAt)
	return s, err
}

const statusSnapshotCols = `id, project_id, stage, project_status, health_status,
	health_summary, total_tasks, done_tasks, open_raid_items, overdue_tasks,
	overdue_milestones, expected_annual_value, realised_annual_value,
	estimated_cost, approved_budget, actual_cost, note, captured_at`

func (s *Store) StatusSnapshot(id int64) (ProjectStatusSnapshot, error) {
	snap, err := scanStatusSnapshot(s.DB.QueryRow("SELECT "+statusSnapshotCols+
		" FROM project_status_snapshots WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return snap, ErrNotFound
	}
	return snap, err
}

// StatusHistory returns newest-first captures decorated with movement from
// the immediately preceding capture.
func (s *Store) StatusHistory(projectID int64, limit int) ([]ProjectStatusSnapshot, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.Query("SELECT "+statusSnapshotCols+` FROM project_status_snapshots
		WHERE project_id = ? ORDER BY id DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectStatusSnapshot
	for rows.Next() {
		snap, err := scanStatusSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	healthRank := map[string]int{"red": 0, "amber": 1, "green": 2, "closed": 2}
	for i := 0; i+1 < len(out); i++ {
		cur, prev := &out[i], out[i+1]
		cur.HasPrevious = true
		cur.TaskCompletionDelta = cur.TaskCompletion() - prev.TaskCompletion()
		cur.OpenRaidDelta = cur.OpenRaidItems - prev.OpenRaidItems
		cur.OverdueDelta = cur.TotalOverdue() - prev.TotalOverdue()
		cur.RealisedValueDelta = cur.RealisedAnnualValue - prev.RealisedAnnualValue
		switch {
		case healthRank[cur.HealthStatus] > healthRank[prev.HealthStatus]:
			cur.HealthMovement = "improved"
		case healthRank[cur.HealthStatus] < healthRank[prev.HealthStatus]:
			cur.HealthMovement = "worsened"
		default:
			cur.HealthMovement = "unchanged"
		}
	}
	return out, nil
}
