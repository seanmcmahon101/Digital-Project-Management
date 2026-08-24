package store

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ProjectTransferRow is the stable tabular representation used for CSV/XLSX
// exports and imports. A matching Code updates; a missing Code creates.
type ProjectTransferRow struct {
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
}

var importCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{0,31}$`)

func ProjectTransferRows(projects []Project) []ProjectTransferRow {
	rows := make([]ProjectTransferRow, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, ProjectTransferRow{
			Code: p.Code, Name: p.Name, Stage: p.Stage, Status: p.Status,
			Sponsor: p.Sponsor, Lead: p.Lead, Department: p.Department,
			ProblemStatement: p.ProblemStatement, Goal: p.Goal, CurrentState: p.CurrentState,
			BusinessCase: p.BusinessCase, ScopeIn: p.ScopeIn, ScopeOut: p.ScopeOut,
			StartDate: p.StartDate, TargetEnd: p.TargetEnd, GoLive: p.GoLive, ClosedAt: p.ClosedAt,
			ClosureSummary: p.ClosureSummary,
		})
	}
	return rows
}

// ImportProjectRows applies a validated batch atomically. Rows with an
// existing code are updated; other rows create a project (preserving a
// supplied code or allocating a DPM code when blank).
func (s *Store) ImportProjectRows(rows []ProjectTransferRow) (created, updated int, err error) {
	if len(rows) == 0 {
		return 0, 0, &ValidationError{Problems: []string{"The import file contains no project rows"}}
	}
	seen := map[string]bool{}
	for i := range rows {
		r := &rows[i]
		r.Code = strings.ToUpper(strings.TrimSpace(r.Code))
		r.Name = strings.TrimSpace(r.Name)
		r.Stage = strings.ToLower(strings.TrimSpace(r.Stage))
		r.Status = strings.ToLower(strings.TrimSpace(r.Status))
		if r.Stage == "" {
			r.Stage = "intake"
		}
		if r.Status == "" {
			r.Status = "active"
		}
		prefix := fmt.Sprintf("Row %d: ", i+2)
		var problems []string
		if r.Name == "" {
			problems = append(problems, prefix+"project name is required")
		}
		if r.Code != "" && !importCodePattern.MatchString(r.Code) {
			problems = append(problems, prefix+"code must contain only letters, numbers, and hyphens")
		}
		if r.Code != "" && seen[r.Code] {
			problems = append(problems, prefix+"duplicate project code "+r.Code)
		}
		seen[r.Code] = r.Code != ""
		if _, ok := StageNames[r.Stage]; !ok {
			problems = append(problems, prefix+"unknown stage "+r.Stage)
		}
		if r.Status != "active" && r.Status != "on_hold" && r.Status != "closed" && r.Status != "cancelled" {
			problems = append(problems, prefix+"unknown status "+r.Status)
		}
		for _, item := range []struct{ value, label string }{
			{r.StartDate, "start_date"}, {r.TargetEnd, "target_end"}, {r.GoLive, "go_live"}, {r.ClosedAt, "closed_at"},
		} {
			if p := ValidDate(strings.TrimSpace(item.value), item.label); p != "" {
				problems = append(problems, prefix+p)
			}
		}
		if len(problems) > 0 {
			return 0, 0, &ValidationError{Problems: problems}
		}
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	for _, r := range rows {
		code := r.Code
		var id int64
		lookupErr := sql.ErrNoRows
		if code != "" {
			lookupErr = tx.QueryRow(`SELECT id FROM projects WHERE code = ?`, code).Scan(&id)
		}
		if lookupErr == nil {
			_, err = tx.Exec(projectImportUpdateSQL(), projectImportArgs(r, id)...)
			if err != nil {
				return created, updated, fmt.Errorf("update %s: %w", code, err)
			}
			updated++
			continue
		}
		if lookupErr != sql.ErrNoRows {
			return created, updated, lookupErr
		}
		if code == "" {
			var next int64
			err = tx.QueryRow(`INSERT INTO ref_counters (project_id, kind, next) VALUES (0, 'DPM', 2)
				ON CONFLICT (project_id, kind) DO UPDATE SET next = next + 1 RETURNING next`).Scan(&next)
			if err != nil {
				return created, updated, err
			}
			code = fmt.Sprintf("DPM-%03d", next-1)
		} else if n := dpmNumber(code); n > 0 {
			_, err = tx.Exec(`INSERT INTO ref_counters (project_id, kind, next) VALUES (0, 'DPM', ?)
				ON CONFLICT (project_id, kind) DO UPDATE SET next = max(next, excluded.next)`, n+1)
			if err != nil {
				return created, updated, err
			}
		}
		args := append([]any{code}, projectImportArgs(r, int64(0))[:17]...)
		_, err = tx.Exec(`INSERT INTO projects (code, name, stage, status, sponsor, lead, department,
			problem_statement, goal, current_state, business_case, scope_in, scope_out,
			start_date, target_end, go_live, closed_at, closure_summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, args...)
		if err != nil {
			return created, updated, fmt.Errorf("create %s: %w", code, err)
		}
		created++
	}
	if err = tx.Commit(); err != nil {
		return created, updated, err
	}
	return created, updated, nil
}

func projectImportUpdateSQL() string {
	return `UPDATE projects SET name=?, stage=?, status=?, sponsor=?, lead=?, department=?,
		problem_statement=?, goal=?, current_state=?, business_case=?, scope_in=?, scope_out=?,
		start_date=?, target_end=?, go_live=?, closed_at=?, closure_summary=?, updated_at=datetime('now') WHERE id=?`
}

func projectImportArgs(r ProjectTransferRow, id int64) []any {
	return []any{r.Name, r.Stage, r.Status, strings.TrimSpace(r.Sponsor), strings.TrimSpace(r.Lead),
		strings.TrimSpace(r.Department), strings.TrimSpace(r.ProblemStatement), strings.TrimSpace(r.Goal),
		strings.TrimSpace(r.CurrentState), strings.TrimSpace(r.BusinessCase), strings.TrimSpace(r.ScopeIn),
		strings.TrimSpace(r.ScopeOut), strings.TrimSpace(r.StartDate), strings.TrimSpace(r.TargetEnd),
		strings.TrimSpace(r.GoLive), strings.TrimSpace(r.ClosedAt), strings.TrimSpace(r.ClosureSummary), id}
}

func dpmNumber(code string) int64 {
	if !strings.HasPrefix(code, "DPM-") {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimPrefix(code, "DPM-"), 10, 64)
	return n
}
