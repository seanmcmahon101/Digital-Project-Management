package store

import (
	"database/sql"
	"errors"
)

type Benefit struct {
	ID            int64
	ProjectID     int64
	Ref           string
	Name          string
	Category      string
	Unit          string
	Direction     string
	BaselineValue sql.NullFloat64
	BaselineDate  string
	TargetValue   sql.NullFloat64
	AnnualValue   float64
	Notes         string
	CreatedAt     string
	UpdatedAt     string
	Measurements  []BenefitMeasurement
	ProjectCode   string
	ProjectName   string
}

type BenefitMeasurement struct {
	ID         int64
	BenefitID  int64
	Value      float64
	MeasuredAt string
	Notes      string
	CreatedAt  string
}

var BenefitCategories = []string{
	"hours_saved", "cost_saved", "error_reduction", "cycle_time",
	"quality", "compliance", "reporting_speed", "adoption", "custom",
}

var BenefitCategoryNames = map[string]string{
	"hours_saved":     "Hours saved",
	"cost_saved":      "Cost saved",
	"error_reduction": "Error reduction",
	"cycle_time":      "Cycle time",
	"quality":         "Quality",
	"compliance":      "Compliance",
	"reporting_speed": "Reporting speed",
	"adoption":        "Adoption",
	"custom":          "Custom",
}

func (b Benefit) CategoryName() string { return BenefitCategoryNames[b.Category] }

// Latest returns the most recent measurement, or nil.
func (b Benefit) Latest() *BenefitMeasurement {
	if len(b.Measurements) == 0 {
		return nil
	}
	return &b.Measurements[len(b.Measurements)-1]
}

// HasBaseline reports whether the before-picture has been captured.
func (b Benefit) HasBaseline() bool { return b.BaselineValue.Valid }

// HasTarget reports whether a target has been set.
func (b Benefit) HasTarget() bool { return b.TargetValue.Valid }

// Progress returns achievement toward target as a 0-100+ percentage, and
// ok=false when baseline/target/measurement are missing or degenerate.
func (b Benefit) Progress() (pct float64, ok bool) {
	last := b.Latest()
	if last == nil || !b.HasBaseline() || !b.HasTarget() {
		return 0, false
	}
	base, target := b.BaselineValue.Float64, b.TargetValue.Float64
	if base == target {
		return 0, false
	}
	pct = (last.Value - base) / (target - base) * 100
	if pct < 0 {
		pct = 0
	}
	return pct, true
}

// Achieved reports whether the latest measurement meets or beats the target.
func (b Benefit) Achieved() bool {
	last := b.Latest()
	if last == nil || !b.HasTarget() {
		return false
	}
	if b.Direction == "increase" {
		return last.Value >= b.TargetValue.Float64
	}
	return last.Value <= b.TargetValue.Float64
}

// RealisedAnnualValue estimates the delivered GBP/year: the expected annual
// value scaled by measured progress (capped at 100%).
func (b Benefit) RealisedAnnualValue() float64 {
	pct, ok := b.Progress()
	if !ok || b.AnnualValue == 0 {
		return 0
	}
	if pct > 100 {
		pct = 100
	}
	return b.AnnualValue * pct / 100
}

// MonthlyHoursSaved estimates hours/month saved so far for hours_saved
// benefits measured in hours (baseline - latest for decrease direction).
func (b Benefit) MonthlyHoursSaved() float64 {
	if b.Category != "hours_saved" {
		return 0
	}
	last := b.Latest()
	if last == nil || !b.HasBaseline() {
		return 0
	}
	saved := b.BaselineValue.Float64 - last.Value
	if b.Direction == "increase" {
		saved = last.Value - b.BaselineValue.Float64
	}
	if saved < 0 {
		return 0
	}
	return saved
}

const benefitCols = `b.id, b.project_id, b.ref, b.name, b.category, b.unit, b.direction,
	b.baseline_value, b.baseline_date, b.target_value, b.annual_value, b.notes,
	b.created_at, b.updated_at`

func (s *Store) Benefit(id int64) (Benefit, error) {
	var b Benefit
	err := s.DB.QueryRow(`SELECT `+benefitCols+` FROM benefits b WHERE b.id = ?`, id).
		Scan(&b.ID, &b.ProjectID, &b.Ref, &b.Name, &b.Category, &b.Unit, &b.Direction,
			&b.BaselineValue, &b.BaselineDate, &b.TargetValue, &b.AnnualValue, &b.Notes,
			&b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	if err != nil {
		return b, err
	}
	b.Measurements, err = s.measurements(id)
	return b, err
}

func (s *Store) measurements(benefitID int64) ([]BenefitMeasurement, error) {
	rows, err := s.DB.Query(`SELECT id, benefit_id, value, measured_at, notes, created_at
		FROM benefit_measurements WHERE benefit_id = ? ORDER BY measured_at, id`, benefitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BenefitMeasurement
	for rows.Next() {
		var m BenefitMeasurement
		if err := rows.Scan(&m.ID, &m.BenefitID, &m.Value, &m.MeasuredAt, &m.Notes, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Benefits returns a project's benefits with measurements attached.
func (s *Store) Benefits(projectID int64) ([]Benefit, error) {
	return s.benefitsWhere(`WHERE b.project_id = ?`, projectID)
}

// BenefitsAllProjects returns benefits across all projects with project
// names attached.
func (s *Store) BenefitsAllProjects() ([]Benefit, error) {
	rows, err := s.DB.Query(`SELECT ` + benefitCols + `, p.code, p.name
		FROM benefits b JOIN projects p ON p.id = b.project_id ORDER BY b.project_id, b.ref`)
	if err != nil {
		return nil, err
	}
	var out []Benefit
	for rows.Next() {
		var b Benefit
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.Ref, &b.Name, &b.Category, &b.Unit,
			&b.Direction, &b.BaselineValue, &b.BaselineDate, &b.TargetValue, &b.AnnualValue,
			&b.Notes, &b.CreatedAt, &b.UpdatedAt, &b.ProjectCode, &b.ProjectName); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.attachMeasurements(out)
}

func (s *Store) benefitsWhere(where string, args ...any) ([]Benefit, error) {
	rows, err := s.DB.Query(`SELECT `+benefitCols+` FROM benefits b `+where+` ORDER BY b.ref`, args...)
	if err != nil {
		return nil, err
	}
	var out []Benefit
	for rows.Next() {
		var b Benefit
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.Ref, &b.Name, &b.Category, &b.Unit,
			&b.Direction, &b.BaselineValue, &b.BaselineDate, &b.TargetValue, &b.AnnualValue,
			&b.Notes, &b.CreatedAt, &b.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.attachMeasurements(out)
}

func (s *Store) attachMeasurements(benefits []Benefit) ([]Benefit, error) {
	if len(benefits) == 0 {
		return benefits, nil
	}
	index := map[int64]int{}
	for i, b := range benefits {
		index[b.ID] = i
	}
	rows, err := s.DB.Query(`SELECT id, benefit_id, value, measured_at, notes, created_at
		FROM benefit_measurements ORDER BY measured_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m BenefitMeasurement
		if err := rows.Scan(&m.ID, &m.BenefitID, &m.Value, &m.MeasuredAt, &m.Notes, &m.CreatedAt); err != nil {
			return nil, err
		}
		if i, ok := index[m.BenefitID]; ok {
			benefits[i].Measurements = append(benefits[i].Measurements, m)
		}
	}
	return benefits, rows.Err()
}

// CreateBenefit defines a new benefit. baseline/target may be nil when not
// yet known.
func (s *Store) CreateBenefit(projectID int64, name, category, unit, direction string,
	baseline, target *float64, baselineDate string, annualValue float64, notes string) error {
	if err := Validate(Require(name, "Benefit name"), ValidDate(baselineDate, "Baseline date")); err != nil {
		return err
	}
	if _, ok := BenefitCategoryNames[category]; !ok {
		category = "custom"
	}
	if direction != "increase" {
		direction = "decrease"
	}
	ref, err := s.NextRef(projectID, "BEN")
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT INTO benefits (project_id, ref, name, category, unit, direction,
		baseline_value, baseline_date, target_value, annual_value, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		projectID, ref, name, category, unit, direction,
		nullable(baseline), baselineDate, nullable(target), annualValue, notes)
	if err == nil {
		s.LogActivity(projectID, "benefit", ref, "defined", name)
	}
	return err
}

func nullable(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func (s *Store) UpdateBenefit(id int64, name, category, unit, direction string,
	baseline, target *float64, baselineDate string, annualValue float64, notes string) error {
	if err := Validate(Require(name, "Benefit name"), ValidDate(baselineDate, "Baseline date")); err != nil {
		return err
	}
	if _, ok := BenefitCategoryNames[category]; !ok {
		category = "custom"
	}
	if direction != "increase" {
		direction = "decrease"
	}
	_, err := s.DB.Exec(`UPDATE benefits SET name=?, category=?, unit=?, direction=?,
		baseline_value=?, baseline_date=?, target_value=?, annual_value=?, notes=?,
		updated_at = datetime('now') WHERE id = ?`,
		name, category, unit, direction, nullable(baseline), baselineDate, nullable(target),
		annualValue, notes, id)
	return err
}

// AddMeasurement records an actual value for a benefit.
func (s *Store) AddMeasurement(benefitID int64, value float64, measuredAt, notes string) error {
	if measuredAt == "" {
		measuredAt = Today()
	}
	if err := Validate(ValidDate(measuredAt, "Measurement date")); err != nil {
		return err
	}
	_, err := s.DB.Exec(`INSERT INTO benefit_measurements (benefit_id, value, measured_at, notes)
		VALUES (?, ?, ?, ?)`, benefitID, value, measuredAt, notes)
	if err != nil {
		return err
	}
	var projectID int64
	var ref string
	if s.DB.QueryRow(`SELECT project_id, ref FROM benefits WHERE id = ?`, benefitID).Scan(&projectID, &ref) == nil {
		s.LogActivity(projectID, "benefit", ref, "measured", "")
	}
	return nil
}

func (s *Store) DeleteMeasurement(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM benefit_measurements WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteBenefit(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM benefits WHERE id = ?`, id)
	return err
}
