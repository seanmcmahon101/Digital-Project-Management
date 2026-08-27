package web

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/coach"
	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
)

// validTabs maps workspace tab slugs to their template names.
var validTabs = map[string]string{
	"overview":       "p_overview",
	"discovery":      "p_discovery",
	"case":           "p_case",
	"plan":           "p_plan",
	"board":          "p_board",
	"requirements":   "p_requirements",
	"raid":           "p_raid",
	"people":         "p_people",
	"decisions":      "p_decisions",
	"changes":        "p_changes",
	"implementation": "p_implementation",
	"benefits":       "p_benefits",
	"close":          "p_close",
	"activity":       "p_activity",
	"documents":      "p_documents",
}

// stageTabs marks which tabs are most relevant per stage, used for the
// progressive-disclosure dot on the tab bar.
var stageTabs = map[string][]string{
	"intake":    {"overview"},
	"discovery": {"discovery", "people", "benefits"},
	"define":    {"case", "requirements", "benefits"},
	"plan":      {"plan", "board", "raid", "people"},
	"build":     {"board", "requirements", "changes", "raid"},
	"implement": {"implementation", "board", "raid"},
	"benefits":  {"benefits", "close"},
}

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	list, err := s.St.Projects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	type row struct {
		P      store.Project
		Health coach.Health
		Next   string
	}
	var rows []row
	for _, p := range list {
		snap, err := s.St.LoadSnapshot(p.ID)
		if err != nil {
			s.serverError(w, err)
			return
		}
		next := ""
		if a, ok := coach.NextAction(snap); ok {
			next = a.Message
		}
		rows = append(rows, row{P: p, Health: coach.Assess(snap), Next: next})
	}
	s.render(w, r, "projects", view{
		Title: "Projects", Active: "projects",
		Data: map[string]any{"Rows": rows},
	})
}

func (s *Server) newProjectForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "project_new", view{Title: "New project", Active: "projects",
		Data: map[string]any{
			"Name":    r.URL.Query().Get("name"),
			"Problem": r.URL.Query().Get("problem"),
		}})
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	p, err := s.St.CreateProject(
		r.FormValue("name"), r.FormValue("sponsor"), r.FormValue("lead"),
		r.FormValue("department"), r.FormValue("problem_statement"), r.FormValue("goal"))
	if err != nil {
		s.fail(w, r, "/projects/new", err)
		return
	}
	s.redirect(w, r, "/projects/"+itoa64(p.ID)+"/overview",
		p.Code+" created. The coach will guide you through Intake — start with the gate checklist below.")
}

func (s *Server) projectHome(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	http.Redirect(w, r, "/projects/"+itoa64(id)+"/overview", http.StatusSeeOther)
}

// projectView loads everything the workspace templates need.
func (s *Server) projectView(id int64) (map[string]any, error) {
	snap, err := s.St.LoadSnapshot(id)
	if err != nil {
		return nil, err
	}
	gate := coach.CheckGate(snap)
	health := coach.Assess(snap)
	advice := coach.Advise(snap)
	doneTasks := 0
	for _, t := range snap.Tasks {
		if t.Status == "done" {
			doneTasks++
		}
	}
	financials := store.SummariseFinancials(snap.Financials, snap.Benefits)
	return map[string]any{
		"Snap":       snap,
		"P":          snap.Project,
		"Gate":       gate,
		"Health":     health,
		"Advice":     advice,
		"StageTabs":  stageTabs[snap.Project.Stage],
		"DoneTasks":  doneTasks,
		"Financials": financials,
	}, nil
}

func (s *Server) projectTab(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	tab := r.PathValue("tab")
	page, ok := validTabs[tab]
	if !ok {
		s.notFound(w)
		return
	}
	data, err := s.projectView(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.notFound(w)
			return
		}
		s.serverError(w, err)
		return
	}
	data["Tab"] = tab

	// Tab-specific extras.
	switch tab {
	case "plan":
		snap := data["Snap"].(*store.Snapshot)
		data["Timeline"] = buildTimeline(snap)
	case "activity":
		entries, err := s.St.Activity(id, 200)
		if err != nil {
			s.serverError(w, err)
			return
		}
		data["Entries"] = entries
	case "documents":
		documents, err := s.St.Documents(id)
		if err != nil {
			s.serverError(w, err)
			return
		}
		data["Documents"] = documents
	case "close", "overview":
		hist, err := s.St.GateHistory(id)
		if err != nil {
			s.serverError(w, err)
			return
		}
		data["GateHistory"] = hist
		if tab == "overview" {
			statusHist, err := s.St.StatusHistory(id, 24)
			if err != nil {
				s.serverError(w, err)
				return
			}
			data["StatusHistory"] = statusHist
		}
	}

	p := data["P"].(store.Project)
	s.render(w, r, page, view{Title: p.Name, Active: "projects", Data: data})
}

// updateProject saves whichever whitelisted fields the form posted.
func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, "/projects/"+itoa64(id)+"/overview")
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, back, err)
		return
	}
	fields := map[string]string{}
	for _, col := range []string{"name", "sponsor", "lead", "department", "problem_statement",
		"goal", "current_state", "business_case", "scope_in", "scope_out",
		"start_date", "target_end", "go_live", "closure_summary"} {
		if _, present := r.PostForm[col]; present {
			fields[col] = strings.TrimSpace(r.PostForm.Get(col))
		}
	}
	if name, present := fields["name"]; present && name == "" {
		s.fail(w, r, back, &store.ValidationError{Problems: []string{"Project name is required"}})
		return
	}
	dateLabels := map[string]string{"start_date": "Start date", "target_end": "Target end date", "go_live": "Go-live date"}
	for dateField, label := range dateLabels {
		if v, present := fields[dateField]; present {
			if problem := store.ValidDate(v, label); problem != "" {
				s.fail(w, r, back, &store.ValidationError{Problems: []string{problem}})
				return
			}
		}
	}
	if err := s.St.UpdateProjectFields(id, fields); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.St.LogActivity(id, "project", "", "updated", "Project details updated")
	s.redirect(w, r, back, "Saved.")
}

// advanceGate moves the project to the next stage, recording an override
// when criteria are unmet.
func (s *Server) advanceGate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := "/projects/" + itoa64(id) + "/overview"
	snap, err := s.St.LoadSnapshot(id)
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	gate := coach.CheckGate(snap)
	p, err := s.St.AdvanceStage(id, gate.Unmet(), strings.TrimSpace(r.FormValue("override_reason")))
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	msg := "Moved to " + p.StageName() + "."
	if len(gate.Unmet()) > 0 {
		msg = "Gate overridden — moved to " + p.StageName() + ". The override and its reason are on record."
	}
	// Entering Implement: offer the standard readiness checklist automatically
	// if none exists, so the user is never staring at an empty page.
	if p.Stage == "implement" {
		items, _ := s.St.ReadinessItems(id)
		if len(items) == 0 {
			if err := s.St.SeedReadinessChecklist(id); err == nil {
				msg += " A standard go-live readiness checklist has been added."
			}
		}
	}
	s.redirect(w, r, back, msg)
}

func (s *Server) closeProject(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := "/projects/" + itoa64(id) + "/close"
	cancelled := r.FormValue("cancelled") == "1"
	summary := strings.TrimSpace(r.FormValue("closure_summary"))
	if summary == "" && !cancelled {
		s.fail(w, r, back, &store.ValidationError{Problems: []string{"Write a short closure summary before closing — it is the record that outlives memories"}})
		return
	}
	if err := s.St.CloseProject(id, summary, cancelled); err != nil {
		s.fail(w, r, back, err)
		return
	}
	verb := "closed"
	if cancelled {
		verb = "cancelled"
	}
	s.redirect(w, r, "/projects/"+itoa64(id)+"/overview", "Project "+verb+".")
}

func (s *Server) reopenProject(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.ReopenProject(id); err != nil {
		s.fail(w, r, "/projects/"+itoa64(id)+"/overview", err)
		return
	}
	s.redirect(w, r, "/projects/"+itoa64(id)+"/overview", "Project reopened.")
}

func (s *Server) projectHold(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	hold := r.FormValue("action") == "hold"
	if err := s.St.SetProjectHold(id, hold, r.FormValue("reason")); err != nil {
		s.fail(w, r, projURL(id, "overview"), err)
		return
	}
	message := "Project resumed."
	if hold {
		message = "Project put on hold. The reason is recorded in its activity history."
	}
	s.redirect(w, r, projURL(id, "overview"), message)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	p, err := s.St.Project(id)
	if err != nil {
		s.fail(w, r, "/projects", err)
		return
	}
	if r.FormValue("confirm_code") != p.Code {
		s.fail(w, r, "/projects/"+itoa64(id)+"/close",
			&store.ValidationError{Problems: []string{"Type the project code (" + p.Code + ") to confirm permanent deletion"}})
		return
	}
	if err := s.St.DeleteProject(id); err != nil {
		s.fail(w, r, "/projects", err)
		return
	}
	uploadDir := filepath.Join(s.DataDir, "uploads", itoa64(id))
	if err := os.RemoveAll(uploadDir); err != nil {
		// The database deletion is already committed. Log cleanup failures so a
		// later maintenance pass can remove the harmless orphaned directory.
		log.Printf("remove uploads for deleted project %d: %v", id, err)
	}
	s.redirect(w, r, "/projects", p.Code+" deleted permanently.")
}

// backTo returns a safe same-site redirect target from the "back" form
// value, falling back to def.
func backTo(r *http.Request, def string) string {
	b := r.FormValue("back")
	if strings.HasPrefix(b, "/") && !strings.HasPrefix(b, "//") {
		return b
	}
	return def
}

func itoa64(v int64) string { return strconv.FormatInt(v, 10) }

// timelineRow places one milestone on the plan timeline as a percentage
// position between the project's earliest and latest known dates.
type timelineRow struct {
	M      store.Milestone
	PosPct float64
}

type timelineData struct {
	Rows     []timelineRow
	TodayPct float64
	StartISO string
	EndISO   string
	Valid    bool
}

func buildTimeline(snap *store.Snapshot) timelineData {
	parse := func(iso string) (time.Time, bool) {
		t, err := time.Parse("2006-01-02", iso)
		return t, err == nil
	}
	var min, max time.Time
	consider := func(iso string) {
		t, ok := parse(iso)
		if !ok {
			return
		}
		if min.IsZero() || t.Before(min) {
			min = t
		}
		if max.IsZero() || t.After(max) {
			max = t
		}
	}
	consider(snap.Project.StartDate)
	consider(snap.Project.TargetEnd)
	for _, m := range snap.Milestones {
		consider(m.DueDate)
	}
	consider(store.Today())
	if min.IsZero() || !max.After(min) {
		return timelineData{}
	}
	span := max.Sub(min).Hours()
	pos := func(iso string) float64 {
		t, ok := parse(iso)
		if !ok {
			return -1
		}
		p := t.Sub(min).Hours() / span * 100
		if p < 0 {
			p = 0
		}
		if p > 100 {
			p = 100
		}
		return p
	}
	td := timelineData{Valid: true, StartISO: min.Format("2006-01-02"), EndISO: max.Format("2006-01-02")}
	td.TodayPct = pos(store.Today())
	for _, m := range snap.Milestones {
		if m.DueDate == "" {
			continue
		}
		td.Rows = append(td.Rows, timelineRow{M: m, PosPct: pos(m.DueDate)})
	}
	return td
}
