package web

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"digipm/internal/coach"
	"digipm/internal/db"
	"digipm/internal/store"
)

// portfolioRow couples a project with its computed assessment for
// dashboards and reports.
type portfolioRow struct {
	P       store.Project
	Snap    *store.Snapshot
	Health  coach.Health
	Gate    coach.GateCheck
	Next    string
	Finance store.FinancialSummary
}

// loadPortfolio assesses every project.
func (s *Server) loadPortfolio() ([]portfolioRow, error) {
	projects, err := s.St.Projects()
	if err != nil {
		return nil, err
	}
	var rows []portfolioRow
	for _, p := range projects {
		snap, err := s.St.LoadSnapshot(p.ID)
		if err != nil {
			return nil, err
		}
		row := portfolioRow{P: p, Snap: snap, Health: coach.Assess(snap), Gate: coach.CheckGate(snap),
			Finance: store.SummariseFinancials(snap.Financials, snap.Benefits)}
		if a, ok := coach.NextAction(snap); ok {
			row.Next = a.Message
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	rows, err := s.loadPortfolio()
	if err != nil {
		s.serverError(w, err)
		return
	}
	ideas, err := s.St.Ideas()
	if err != nil {
		s.serverError(w, err)
		return
	}

	active, closed := 0, 0
	byStage := map[string]int{}
	healthCount := map[string]int{}
	var attention []portfolioRow
	var expectedAnnual, realisedAnnual, hoursSavedMonthly float64
	benefitsMeasured := 0
	for _, row := range rows {
		if row.P.IsClosed() {
			closed++
		} else {
			active++
			byStage[row.P.Stage]++
			healthCount[row.Health.Status]++
			if row.Health.Status != "green" {
				attention = append(attention, row)
			}
		}
		for _, b := range row.Snap.Benefits {
			expectedAnnual += b.AnnualValue
			realisedAnnual += b.RealisedAnnualValue()
			hoursSavedMonthly += b.MonthlyHoursSaved()
			if len(b.Measurements) > 0 {
				benefitsMeasured++
			}
		}
	}
	openIdeas := 0
	for _, i := range ideas {
		if i.Status == "new" || i.Status == "scored" || i.Status == "approved" {
			openIdeas++
		}
	}

	milestones, err := s.St.UpcomingMilestones(21)
	if err != nil {
		s.serverError(w, err)
		return
	}
	openTasks, err := s.St.OpenTasksAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	var overdue []store.Task
	for _, t := range openTasks {
		if t.Overdue() {
			overdue = append(overdue, t)
		}
	}
	risks, err := s.St.OpenRisksAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	var majorRisks []store.RaidItem
	for _, rk := range risks {
		if rk.Severity() == "high" {
			majorRisks = append(majorRisks, rk)
		}
	}
	if len(majorRisks) > 6 {
		majorRisks = majorRisks[:6]
	}

	// "What should I do next": the single top action per active project.
	type nextItem struct {
		P      store.Project
		Advice coach.Advice
	}
	var nextActions []nextItem
	for _, row := range rows {
		if row.P.IsClosed() {
			continue
		}
		if a, ok := coach.NextAction(row.Snap); ok {
			nextActions = append(nextActions, nextItem{P: row.P, Advice: a})
		}
	}
	// Most urgent first.
	rank := map[string]int{"act": 0, "soon": 1, "consider": 2}
	for i := 1; i < len(nextActions); i++ {
		for j := i; j > 0 && rank[nextActions[j].Advice.Severity] < rank[nextActions[j-1].Advice.Severity]; j-- {
			nextActions[j], nextActions[j-1] = nextActions[j-1], nextActions[j]
		}
	}

	maxStage := 1
	for _, n := range byStage {
		if n > maxStage {
			maxStage = n
		}
	}

	s.render(w, r, "dashboard", view{
		Title: "Dashboard", Active: "dashboard",
		Data: map[string]any{
			"Rows": rows, "Active": active, "Closed": closed, "OpenIdeas": openIdeas,
			"ByStage": byStage, "MaxStage": maxStage, "HealthCount": healthCount,
			"Attention": attention, "Milestones": milestones, "Overdue": overdue,
			"MajorRisks": majorRisks, "NextActions": nextActions,
			"ExpectedAnnual": expectedAnnual, "RealisedAnnual": realisedAnnual,
			"HoursSavedMonthly": hoursSavedMonthly, "BenefitsMeasured": benefitsMeasured,
			"HasAnything": len(rows) > 0 || len(ideas) > 0,
		},
	})
}

// --- Global lists ---

func (s *Server) globalTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.St.OpenTasksAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "tasks", view{Title: "My work", Active: "tasks",
		Data: map[string]any{"Tasks": tasks}})
}

func (s *Server) globalRisks(w http.ResponseWriter, r *http.Request) {
	risks, err := s.St.OpenRisksAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "risks", view{Title: "Risks & issues", Active: "risks",
		Data: map[string]any{"Risks": risks}})
}

func (s *Server) globalDecisions(w http.ResponseWriter, r *http.Request) {
	decisions, err := s.St.DecisionsAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "decisions", view{Title: "Decisions", Active: "decisions",
		Data: map[string]any{"Decisions": decisions}})
}

func (s *Server) globalBenefits(w http.ResponseWriter, r *http.Request) {
	benefits, err := s.St.BenefitsAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	var expected, realised, hoursMonthly float64
	achieved := 0
	for _, b := range benefits {
		expected += b.AnnualValue
		realised += b.RealisedAnnualValue()
		hoursMonthly += b.MonthlyHoursSaved()
		if b.Achieved() {
			achieved++
		}
	}
	s.render(w, r, "benefits", view{Title: "Benefits", Active: "benefits",
		Data: map[string]any{
			"Benefits": benefits, "Expected": expected, "Realised": realised,
			"HoursMonthly": hoursMonthly, "Achieved": achieved,
		}})
}

func (s *Server) globalLessons(w http.ResponseWriter, r *http.Request) {
	lessons, err := s.St.LessonsAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "lessons", view{Title: "Lessons learned", Active: "lessons",
		Data: map[string]any{"Lessons": lessons}})
}

// --- Reports ---

func (s *Server) reports(w http.ResponseWriter, r *http.Request) {
	rows, err := s.loadPortfolio()
	if err != nil {
		s.serverError(w, err)
		return
	}
	benefits, err := s.St.BenefitsAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	var expected, realised float64
	var investment, approved, actual float64
	for _, b := range benefits {
		expected += b.AnnualValue
		realised += b.RealisedAnnualValue()
	}
	for _, row := range rows {
		investment += row.Finance.Investment
		approved += row.Finance.Financials.ApprovedBudget
		actual += row.Finance.Financials.ActualCost
	}
	portfolioROI, hasPortfolioROI := 0.0, false
	if investment > 0 {
		portfolioROI = (expected - investment) / investment * 100
		hasPortfolioROI = true
	}
	s.render(w, r, "reports", view{Title: "Reports", Active: "reports",
		Data: map[string]any{
			"Rows": rows, "Benefits": benefits,
			"Expected": expected, "Realised": realised,
			"Investment": investment, "ApprovedBudget": approved, "ActualCost": actual,
			"PortfolioROI": portfolioROI, "HasPortfolioROI": hasPortfolioROI,
			"Today": store.Today(),
		}})
}

func (s *Server) projectReport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	data, err := s.projectView(id)
	if err != nil {
		s.notFound(w)
		return
	}
	hist, err := s.St.GateHistory(id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	data["GateHistory"] = hist
	statusHist, err := s.St.StatusHistory(id, 12)
	if err != nil {
		s.serverError(w, err)
		return
	}
	data["StatusHistory"] = statusHist
	data["Today"] = store.Today()
	p := data["P"].(store.Project)
	s.render(w, r, "report_project", view{Title: p.Code + " status report", Active: "reports", Data: data})
}

// --- Settings, backup, restore, demo ---

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "settings", view{Title: "Settings", Active: "settings",
		Data: map[string]any{
			"OrgName":      s.St.Setting("org_name", ""),
			"CurrencyV":    s.St.Setting("currency", "£"),
			"HourlyRate":   s.St.Setting("hourly_rate", "30"),
			"SidebarColor": normaliseHexColor(s.St.Setting("sidebar_color", "#5C1E30")),
			"Backups":      store.ListBackups(s.BackupDir),
			"DataDir":      s.DataDir,
			"Version":      s.Version,
		}})
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	sidebarColor := strings.TrimSpace(r.FormValue("sidebar_color"))
	if normaliseHexColor(sidebarColor) != strings.ToUpper(sidebarColor) {
		s.fail(w, r, "/settings", errValidation("Choose a valid six-digit sidebar colour"))
		return
	}
	pairs := map[string]string{
		"org_name":      strings.TrimSpace(r.FormValue("org_name")),
		"currency":      strings.TrimSpace(r.FormValue("currency")),
		"hourly_rate":   strings.TrimSpace(r.FormValue("hourly_rate")),
		"sidebar_color": strings.ToUpper(sidebarColor),
	}
	if pairs["currency"] == "" {
		pairs["currency"] = "£"
	}
	for k, v := range pairs {
		if err := s.St.SetSetting(k, v); err != nil {
			s.fail(w, r, "/settings", err)
			return
		}
	}
	s.redirect(w, r, "/settings", "Settings saved.")
}

func (s *Server) makeBackup(w http.ResponseWriter, r *http.Request) {
	path, err := s.St.Backup(s.BackupDir)
	if err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	s.redirect(w, r, "/settings", "Backup created: "+filepath.Base(path))
}

func (s *Server) downloadBackup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	// Only allow files that look like our backups; no path traversal.
	if !strings.HasPrefix(name, "backup-") || !strings.HasSuffix(name, ".db") || strings.ContainsAny(name, `/\`) {
		s.notFound(w)
		return
	}
	path := filepath.Join(s.BackupDir, name)
	if _, err := os.Stat(path); err != nil {
		s.notFound(w)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path)
}

// restoreBackup restores from an existing backup in the backups folder.
func (s *Server) restoreBackup(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if !strings.HasPrefix(name, "backup-") || !strings.HasSuffix(name, ".db") || strings.ContainsAny(name, `/\`) {
		s.fail(w, r, "/settings", errValidation("Choose a backup to restore"))
		return
	}
	s.doRestore(w, r, filepath.Join(s.BackupDir, name))
}

// restoreUpload restores from an uploaded database file.
func (s *Server) restoreUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		s.fail(w, r, "/settings", errValidation("Choose a database file to restore"))
		return
	}
	defer file.Close()
	tmp := filepath.Join(s.DataDir, "restore-upload.tmp")
	out, err := os.Create(tmp)
	if err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(tmp)
		s.fail(w, r, "/settings", err)
		return
	}
	out.Close()
	defer os.Remove(tmp)
	s.doRestore(w, r, tmp)
}

// doRestore validates the candidate file, takes a safety backup, swaps
// the database file and reopens the connection.
func (s *Server) doRestore(w http.ResponseWriter, r *http.Request, candidate string) {
	// 1. Validate: it must open as a SQLite db with (or migratable to) our schema.
	check, err := db.Open(candidate)
	if err != nil {
		s.fail(w, r, "/settings", errValidation("That file is not a valid backup of this application"))
		return
	}
	var count int
	err = check.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&count)
	check.Close()
	if err != nil {
		s.fail(w, r, "/settings", errValidation("That file is not a valid backup of this application"))
		return
	}

	// 2. Safety backup of the current database.
	if _, err := s.St.Backup(s.BackupDir); err != nil {
		s.fail(w, r, "/settings", fmt.Errorf("safety backup failed, restore aborted: %w", err))
		return
	}

	// 3. Close, swap the file, reopen.
	if err := s.St.DB.Close(); err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	os.Remove(s.DBPath + "-wal")
	os.Remove(s.DBPath + "-shm")
	if err := copyFile(candidate, s.DBPath); err != nil {
		// Try to reopen whatever is on disk so the app stays alive.
		if reopened, rerr := db.Open(s.DBPath); rerr == nil {
			s.St.DB = reopened
		}
		s.fail(w, r, "/settings", err)
		return
	}
	reopened, err := db.Open(s.DBPath)
	if err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	s.St.DB = reopened
	s.redirect(w, r, "/", "Backup restored. A safety copy of the previous data was kept in the backups folder.")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
