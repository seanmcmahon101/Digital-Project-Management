package store

// Snapshot bundles everything known about one project so the coach, health
// and gate engines can reason over it without further queries.
type Snapshot struct {
	Project        Project
	Milestones     []Milestone
	Tasks          []Task
	Raid           []RaidItem
	Decisions      []Decision
	Stakeholders   []Stakeholder
	Raci           []RaciActivity
	Requirements   []Requirement // includes linked tests
	Tests          []Test
	ScopeItems     []ScopeItem
	ScopeBaselines []ScopeBaseline
	Changes        []ChangeRequest
	PainPoints     []PainPoint
	Benefits       []Benefit // includes measurements
	Financials     ProjectFinancials
	Readiness      []ReadinessItem
	Lessons        []Lesson
	// GateHistory is newest-first; the coach uses it to know how long the
	// project has sat in its current stage.
	GateHistory []GateEntry
	// LastActivityAt is the timestamp of the most recent activity-log
	// entry, or "" when nothing has been recorded.
	LastActivityAt string
}

// LoadSnapshot loads a full project snapshot.
func (s *Store) LoadSnapshot(projectID int64) (*Snapshot, error) {
	p, err := s.Project(projectID)
	if err != nil {
		return nil, err
	}
	snap := &Snapshot{Project: p}
	if snap.Milestones, err = s.Milestones(projectID); err != nil {
		return nil, err
	}
	if snap.Tasks, err = s.Tasks(projectID); err != nil {
		return nil, err
	}
	if snap.Raid, err = s.RaidItems(projectID); err != nil {
		return nil, err
	}
	if snap.Decisions, err = s.Decisions(projectID); err != nil {
		return nil, err
	}
	if snap.Stakeholders, err = s.Stakeholders(projectID); err != nil {
		return nil, err
	}
	if snap.Raci, err = s.RaciActivities(projectID); err != nil {
		return nil, err
	}
	if snap.Requirements, err = s.Requirements(projectID); err != nil {
		return nil, err
	}
	if snap.Tests, err = s.Tests(projectID); err != nil {
		return nil, err
	}
	if snap.ScopeItems, err = s.ScopeItems(projectID); err != nil {
		return nil, err
	}
	if snap.ScopeBaselines, err = s.ScopeBaselines(projectID); err != nil {
		return nil, err
	}
	if snap.Changes, err = s.ChangeRequests(projectID); err != nil {
		return nil, err
	}
	if snap.PainPoints, err = s.PainPoints(projectID); err != nil {
		return nil, err
	}
	if snap.Benefits, err = s.Benefits(projectID); err != nil {
		return nil, err
	}
	if snap.Financials, err = s.Financials(projectID); err != nil {
		return nil, err
	}
	if snap.Readiness, err = s.ReadinessItems(projectID); err != nil {
		return nil, err
	}
	if snap.Lessons, err = s.Lessons(projectID); err != nil {
		return nil, err
	}
	if snap.GateHistory, err = s.GateHistory(projectID); err != nil {
		return nil, err
	}
	if err := s.DB.QueryRow(`SELECT COALESCE(MAX(created_at), '') FROM activity_log
		WHERE project_id = ?`, projectID).Scan(&snap.LastActivityAt); err != nil {
		return nil, err
	}
	return snap, nil
}
