package store

type Stakeholder struct {
	ID        int64
	ProjectID int64
	Name      string
	Role      string
	Influence string
	Interest  string
	Attitude  string
	Notes     string
	CreatedAt string
}

func (s *Store) Stakeholders(projectID int64) ([]Stakeholder, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, name, role, influence, interest, attitude, notes, created_at
		FROM stakeholders WHERE project_id = ?
		ORDER BY CASE influence WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Stakeholder
	for rows.Next() {
		var st Stakeholder
		if err := rows.Scan(&st.ID, &st.ProjectID, &st.Name, &st.Role, &st.Influence,
			&st.Interest, &st.Attitude, &st.Notes, &st.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func normalizeLevel(v string) string {
	switch v {
	case "low", "high":
		return v
	default:
		return "medium"
	}
}

func (s *Store) CreateStakeholder(projectID int64, name, role, influence, interest, attitude, notes string) error {
	if err := Validate(Require(name, "Name")); err != nil {
		return err
	}
	switch attitude {
	case "champion", "supportive", "resistant":
	default:
		attitude = "neutral"
	}
	_, err := s.DB.Exec(`INSERT INTO stakeholders (project_id, name, role, influence, interest, attitude, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		projectID, name, role, normalizeLevel(influence), normalizeLevel(interest), attitude, notes)
	if err == nil {
		s.LogActivity(projectID, "stakeholder", "", "added", name)
	}
	return err
}

func (s *Store) UpdateStakeholder(id int64, name, role, influence, interest, attitude, notes string) error {
	if err := Validate(Require(name, "Name")); err != nil {
		return err
	}
	switch attitude {
	case "champion", "supportive", "resistant":
	default:
		attitude = "neutral"
	}
	_, err := s.DB.Exec(`UPDATE stakeholders SET name=?, role=?, influence=?, interest=?, attitude=?, notes=?
		WHERE id = ?`, name, role, normalizeLevel(influence), normalizeLevel(interest), attitude, notes, id)
	return err
}

func (s *Store) DeleteStakeholder(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM stakeholders WHERE id = ?`, id)
	return err
}

// --- RACI ---

type RaciActivity struct {
	ID        int64
	ProjectID int64
	Name      string
	SortOrder int
	// Letters maps stakeholder id -> R/A/C/I (absent = no involvement).
	Letters map[int64]string
}

func (s *Store) RaciActivities(projectID int64) ([]RaciActivity, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, name, sort_order FROM raci_activities
		WHERE project_id = ? ORDER BY sort_order, id`, projectID)
	if err != nil {
		return nil, err
	}
	var out []RaciActivity
	index := map[int64]int{}
	for rows.Next() {
		var a RaciActivity
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.SortOrder); err != nil {
			rows.Close()
			return nil, err
		}
		a.Letters = map[int64]string{}
		index[a.ID] = len(out)
		out = append(out, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	arows, err := s.DB.Query(`SELECT ra.activity_id, ra.stakeholder_id, ra.letter
		FROM raci_assignments ra
		JOIN raci_activities a ON a.id = ra.activity_id
		WHERE a.project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer arows.Close()
	for arows.Next() {
		var actID, stID int64
		var letter string
		if err := arows.Scan(&actID, &stID, &letter); err != nil {
			return nil, err
		}
		if i, ok := index[actID]; ok {
			out[i].Letters[stID] = letter
		}
	}
	return out, arows.Err()
}

func (s *Store) CreateRaciActivity(projectID int64, name string) error {
	if err := Validate(Require(name, "Activity name")); err != nil {
		return err
	}
	_, err := s.DB.Exec(`INSERT INTO raci_activities (project_id, name, sort_order)
		VALUES (?, ?, COALESCE((SELECT MAX(sort_order)+1 FROM raci_activities WHERE project_id = ?), 0))`,
		projectID, name, projectID)
	return err
}

// SetRaci assigns or clears a RACI letter for one activity/stakeholder cell.
func (s *Store) SetRaci(activityID, stakeholderID int64, letter string) error {
	var sameProject int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM raci_activities a
		JOIN stakeholders sh ON sh.id = ? AND sh.project_id = a.project_id
		WHERE a.id = ?`, stakeholderID, activityID).Scan(&sameProject); err != nil {
		return err
	}
	if sameProject == 0 {
		return &ValidationError{Problems: []string{"The activity and stakeholder must belong to the same project"}}
	}
	if letter == "" {
		_, err := s.DB.Exec(`DELETE FROM raci_assignments WHERE activity_id=? AND stakeholder_id=?`,
			activityID, stakeholderID)
		return err
	}
	switch letter {
	case "R", "A", "C", "I":
	default:
		return &ValidationError{Problems: []string{"RACI value must be R, A, C or I"}}
	}
	_, err := s.DB.Exec(`INSERT INTO raci_assignments (activity_id, stakeholder_id, letter)
		VALUES (?, ?, ?) ON CONFLICT (activity_id, stakeholder_id) DO UPDATE SET letter = excluded.letter`,
		activityID, stakeholderID, letter)
	return err
}

func (s *Store) DeleteRaciActivity(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM raci_activities WHERE id = ?`, id)
	return err
}
