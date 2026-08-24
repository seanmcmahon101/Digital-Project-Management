package store

// --- Pain points (discovery) ---

type PainPoint struct {
	ID          int64
	ProjectID   int64
	Description string
	ProcessArea string
	Impact      string
	Frequency   string
	CreatedAt   string
}

func (s *Store) PainPoints(projectID int64) ([]PainPoint, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, description, process_area, impact, frequency, created_at
		FROM pain_points WHERE project_id = ?
		ORDER BY CASE impact WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PainPoint
	for rows.Next() {
		var p PainPoint
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.Description, &p.ProcessArea, &p.Impact, &p.Frequency, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreatePainPoint(projectID int64, description, processArea, impact, frequency string) error {
	if err := Validate(Require(description, "Pain point description")); err != nil {
		return err
	}
	_, err := s.DB.Exec(`INSERT INTO pain_points (project_id, description, process_area, impact, frequency)
		VALUES (?, ?, ?, ?, ?)`, projectID, description, processArea, normalizeLevel(impact), frequency)
	if err == nil {
		s.LogActivity(projectID, "pain_point", "", "captured", description)
	}
	return err
}

func (s *Store) DeletePainPoint(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM pain_points WHERE id = ?`, id)
	return err
}

// --- Implementation readiness ---

type ReadinessItem struct {
	ID        int64
	ProjectID int64
	Category  string
	Item      string
	Owner     string
	Done      bool
	Notes     string
	SortOrder int
	CreatedAt string
}

var ReadinessCategories = []string{"training", "communications", "support", "data", "technical", "rollback"}

var ReadinessCategoryNames = map[string]string{
	"training":       "Training",
	"communications": "Communications",
	"support":        "Support",
	"data":           "Data",
	"technical":      "Technical",
	"rollback":       "Rollback plan",
}

func (r ReadinessItem) CategoryName() string { return ReadinessCategoryNames[r.Category] }

func (s *Store) ReadinessItems(projectID int64) ([]ReadinessItem, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, category, item, owner, done, notes, sort_order, created_at
		FROM readiness_items WHERE project_id = ? ORDER BY sort_order, id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReadinessItem
	for rows.Next() {
		var r ReadinessItem
		var done int
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Category, &r.Item, &r.Owner, &done, &r.Notes, &r.SortOrder, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Done = done == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateReadinessItem(projectID int64, category, item, owner string) error {
	if err := Validate(Require(item, "Checklist item")); err != nil {
		return err
	}
	if _, ok := ReadinessCategoryNames[category]; !ok {
		category = "technical"
	}
	_, err := s.DB.Exec(`INSERT INTO readiness_items (project_id, category, item, owner, sort_order)
		VALUES (?, ?, ?, ?, COALESCE((SELECT MAX(sort_order)+1 FROM readiness_items WHERE project_id = ?), 0))`,
		projectID, category, item, owner, projectID)
	return err
}

// SeedReadinessChecklist adds a standard go-live checklist to a project.
// Existing items are kept; only missing standard items are added.
func (s *Store) SeedReadinessChecklist(projectID int64) error {
	standard := []struct{ category, item string }{
		{"training", "Identify everyone who will use the new solution"},
		{"training", "Prepare training material or quick-reference guide"},
		{"training", "Run training sessions and record attendance"},
		{"communications", "Announce the go-live date and what changes for each team"},
		{"communications", "Agree the cutover plan with affected departments"},
		{"support", "Agree who provides first-line support after go-live"},
		{"support", "Set up a way for users to report problems"},
		{"data", "Migrate or load required data and verify accuracy"},
		{"technical", "Confirm access and permissions for all users"},
		{"technical", "Complete final end-to-end test in the live environment"},
		{"rollback", "Document how to revert to the old process if go-live fails"},
	}
	existing := map[string]bool{}
	rows, err := s.DB.Query(`SELECT item FROM readiness_items WHERE project_id = ?`, projectID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item string
		if err := rows.Scan(&item); err != nil {
			rows.Close()
			return err
		}
		existing[item] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, it := range standard {
		if existing[it.item] {
			continue
		}
		if err := s.CreateReadinessItem(projectID, it.category, it.item, ""); err != nil {
			return err
		}
	}
	s.LogActivity(projectID, "readiness", "", "seeded", "Standard go-live checklist added")
	return nil
}

func (s *Store) SetReadinessDone(id int64, done bool) error {
	v := 0
	if done {
		v = 1
	}
	_, err := s.DB.Exec(`UPDATE readiness_items SET done = ? WHERE id = ?`, v, id)
	return err
}

func (s *Store) DeleteReadinessItem(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM readiness_items WHERE id = ?`, id)
	return err
}

// --- Lessons learned ---

type Lesson struct {
	ID             int64
	ProjectID      int64
	Category       string
	Lesson         string
	Recommendation string
	CreatedAt      string
	ProjectCode    string
	ProjectName    string
}

var LessonCategoryNames = map[string]string{
	"went_well": "What went well", "went_wrong": "What went wrong", "do_differently": "Do differently next time",
}

func (l Lesson) CategoryName() string { return LessonCategoryNames[l.Category] }

func (s *Store) Lessons(projectID int64) ([]Lesson, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, category, lesson, recommendation, created_at
		FROM lessons WHERE project_id = ? ORDER BY category, id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Lesson
	for rows.Next() {
		var l Lesson
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Category, &l.Lesson, &l.Recommendation, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// LessonsAllProjects returns every lesson with its project attached, newest first.
func (s *Store) LessonsAllProjects() ([]Lesson, error) {
	rows, err := s.DB.Query(`SELECT l.id, l.project_id, l.category, l.lesson, l.recommendation,
		l.created_at, p.code, p.name
		FROM lessons l JOIN projects p ON p.id = l.project_id ORDER BY l.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Lesson
	for rows.Next() {
		var l Lesson
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Category, &l.Lesson, &l.Recommendation,
			&l.CreatedAt, &l.ProjectCode, &l.ProjectName); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) CreateLesson(projectID int64, category, lesson, recommendation string) error {
	if err := Validate(Require(lesson, "Lesson")); err != nil {
		return err
	}
	if _, ok := LessonCategoryNames[category]; !ok {
		category = "went_well"
	}
	_, err := s.DB.Exec(`INSERT INTO lessons (project_id, category, lesson, recommendation)
		VALUES (?, ?, ?, ?)`, projectID, category, lesson, recommendation)
	if err == nil {
		s.LogActivity(projectID, "lesson", "", "recorded", lesson)
	}
	return err
}

func (s *Store) DeleteLesson(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM lessons WHERE id = ?`, id)
	return err
}
