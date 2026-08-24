package store

import (
	"database/sql"
	"errors"
	"fmt"
)

type Task struct {
	ID          int64
	ProjectID   int64
	Ref         string
	Title       string
	Notes       string
	Status      string
	Priority    string
	Assignee    string
	DueDate     string
	MilestoneID sql.NullInt64
	CompletedAt string
	CreatedAt   string
	UpdatedAt   string
	// Joined for cross-project views.
	ProjectCode string
	ProjectName string
}

// Overdue reports whether the task has a due date in the past and is not done.
func (t Task) Overdue() bool {
	if t.Status == "done" {
		return false
	}
	d, ok := DaysUntil(t.DueDate)
	return ok && d < 0
}

var TaskStatuses = []string{"todo", "doing", "blocked", "done"}

var TaskStatusNames = map[string]string{
	"todo": "To do", "doing": "In progress", "blocked": "Blocked", "done": "Done",
}

const taskCols = `t.id, t.project_id, t.ref, t.title, t.notes, t.status, t.priority,
	t.assignee, t.due_date, t.milestone_id, t.completed_at, t.created_at, t.updated_at`

func scanTask(row interface{ Scan(...any) error }) (Task, error) {
	var t Task
	err := row.Scan(&t.ID, &t.ProjectID, &t.Ref, &t.Title, &t.Notes, &t.Status, &t.Priority,
		&t.Assignee, &t.DueDate, &t.MilestoneID, &t.CompletedAt, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (s *Store) Task(id int64) (Task, error) {
	t, err := scanTask(s.DB.QueryRow(`SELECT `+taskCols+` FROM tasks t WHERE t.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

func (s *Store) Tasks(projectID int64) ([]Task, error) {
	rows, err := s.DB.Query(`SELECT `+taskCols+` FROM tasks t WHERE t.project_id = ?
		ORDER BY CASE t.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, t.id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// OpenTasksAllProjects returns not-done tasks across active projects,
// soonest due first (undated last).
func (s *Store) OpenTasksAllProjects() ([]Task, error) {
	rows, err := s.DB.Query(`SELECT ` + taskCols + `, p.code, p.name
		FROM tasks t JOIN projects p ON p.id = t.project_id
		WHERE t.status != 'done' AND p.status = 'active'
		ORDER BY CASE WHEN t.due_date = '' THEN 1 ELSE 0 END, t.due_date,
		CASE t.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Ref, &t.Title, &t.Notes, &t.Status,
			&t.Priority, &t.Assignee, &t.DueDate, &t.MilestoneID, &t.CompletedAt,
			&t.CreatedAt, &t.UpdatedAt, &t.ProjectCode, &t.ProjectName); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CreateTask(projectID int64, title, notes, priority, assignee, dueDate string, milestoneID int64) (Task, error) {
	if err := Validate(Require(title, "Task title"), ValidDate(dueDate, "Due date")); err != nil {
		return Task{}, err
	}
	if priority != "low" && priority != "high" {
		priority = "medium"
	}
	ref, err := s.NextRef(projectID, "TASK")
	if err != nil {
		return Task{}, err
	}
	var ms any
	if milestoneID > 0 {
		ms = milestoneID
	}
	res, err := s.DB.Exec(`INSERT INTO tasks (project_id, ref, title, notes, priority, assignee, due_date, milestone_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, projectID, ref, title, notes, priority, assignee, dueDate, ms)
	if err != nil {
		return Task{}, err
	}
	id, _ := res.LastInsertId()
	s.LogActivity(projectID, "task", ref, "created", title)
	return s.Task(id)
}

func (s *Store) UpdateTask(id int64, title, notes, priority, assignee, dueDate string, milestoneID int64) error {
	if err := Validate(Require(title, "Task title"), ValidDate(dueDate, "Due date")); err != nil {
		return err
	}
	var ms any
	if milestoneID > 0 {
		ms = milestoneID
	}
	_, err := s.DB.Exec(`UPDATE tasks SET title=?, notes=?, priority=?, assignee=?, due_date=?,
		milestone_id=?, updated_at = datetime('now') WHERE id = ?`,
		title, notes, priority, assignee, dueDate, ms, id)
	return err
}

// SetTaskStatus moves a task on the board, stamping completion when done.
func (s *Store) SetTaskStatus(id int64, status string) error {
	valid := false
	for _, st := range TaskStatuses {
		if st == status {
			valid = true
		}
	}
	if !valid {
		return &ValidationError{Problems: []string{"Invalid task status"}}
	}
	completed := ""
	if status == "done" {
		completed = Today()
	}
	_, err := s.DB.Exec(`UPDATE tasks SET status=?, completed_at=?, updated_at = datetime('now')
		WHERE id = ?`, status, completed, id)
	if err != nil {
		return err
	}
	if t, terr := s.Task(id); terr == nil {
		s.LogActivity(t.ProjectID, "task", t.Ref, "status_"+status, t.Title)
	}
	return nil
}

func (s *Store) DeleteTask(id int64) error {
	t, err := s.Task(id)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err == nil {
		s.LogActivity(t.ProjectID, "task", t.Ref, "deleted", t.Title)
	}
	return err
}

// --- Milestones ---

type Milestone struct {
	ID          int64
	ProjectID   int64
	Name        string
	DueDate     string
	CompletedAt string
	Notes       string
	CreatedAt   string
	ProjectCode string
	ProjectName string
}

func (m Milestone) Done() bool { return m.CompletedAt != "" }
func (m Milestone) Overdue() bool {
	if m.Done() {
		return false
	}
	d, ok := DaysUntil(m.DueDate)
	return ok && d < 0
}

func (s *Store) Milestones(projectID int64) ([]Milestone, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, name, due_date, completed_at, notes, created_at
		FROM milestones WHERE project_id = ?
		ORDER BY CASE WHEN due_date = '' THEN 1 ELSE 0 END, due_date, id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Milestone
	for rows.Next() {
		var m Milestone
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Name, &m.DueDate, &m.CompletedAt, &m.Notes, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpcomingMilestones returns incomplete milestones across active projects
// due within the next horizon days (or overdue).
func (s *Store) UpcomingMilestones(horizonDays int) ([]Milestone, error) {
	rows, err := s.DB.Query(`SELECT m.id, m.project_id, m.name, m.due_date, m.completed_at, m.notes,
		m.created_at, p.code, p.name
		FROM milestones m JOIN projects p ON p.id = m.project_id
		WHERE m.completed_at = '' AND m.due_date != '' AND p.status = 'active'
		AND m.due_date <= date('now', ?)
		ORDER BY m.due_date`, fmt.Sprintf("+%d days", horizonDays))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Milestone
	for rows.Next() {
		var m Milestone
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Name, &m.DueDate, &m.CompletedAt, &m.Notes,
			&m.CreatedAt, &m.ProjectCode, &m.ProjectName); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) CreateMilestone(projectID int64, name, dueDate, notes string) error {
	if err := Validate(Require(name, "Milestone name"), ValidDate(dueDate, "Due date")); err != nil {
		return err
	}
	_, err := s.DB.Exec(`INSERT INTO milestones (project_id, name, due_date, notes) VALUES (?, ?, ?, ?)`,
		projectID, name, dueDate, notes)
	if err == nil {
		s.LogActivity(projectID, "milestone", "", "created", name)
	}
	return err
}

func (s *Store) UpdateMilestone(id int64, name, dueDate, notes string) error {
	if err := Validate(Require(name, "Milestone name"), ValidDate(dueDate, "Due date")); err != nil {
		return err
	}
	_, err := s.DB.Exec(`UPDATE milestones SET name=?, due_date=?, notes=? WHERE id = ?`,
		name, dueDate, notes, id)
	return err
}

// SetMilestoneDone toggles completion.
func (s *Store) SetMilestoneDone(id int64, done bool) error {
	completed := ""
	if done {
		completed = Today()
	}
	_, err := s.DB.Exec(`UPDATE milestones SET completed_at = ? WHERE id = ?`, completed, id)
	if err != nil {
		return err
	}
	var projectID int64
	var name string
	if s.DB.QueryRow(`SELECT project_id, name FROM milestones WHERE id = ?`, id).Scan(&projectID, &name) == nil {
		action := "reopened"
		if done {
			action = "completed"
		}
		s.LogActivity(projectID, "milestone", "", action, name)
	}
	return nil
}

func (s *Store) DeleteMilestone(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM milestones WHERE id = ?`, id)
	return err
}
