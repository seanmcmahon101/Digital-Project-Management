package web

import (
	"net/http"
	"strconv"
	"strings"

	"digipm/internal/store"
)

// projRedirect sends back to a workspace tab of the project.
func projURL(projectID int64, tab string) string {
	return "/projects/" + itoa64(projectID) + "/" + tab
}

// --- Tasks ---

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "board"))
	t, err := s.St.CreateTask(id, r.FormValue("title"), r.FormValue("notes"),
		r.FormValue("priority"), r.FormValue("assignee"), r.FormValue("due_date"),
		formInt64(r, "milestone_id"))
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, t.Ref+" added.")
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	t, err := s.St.Task(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(t.ProjectID, "board"))
	if err := s.St.UpdateTask(id, r.FormValue("title"), r.FormValue("notes"),
		r.FormValue("priority"), r.FormValue("assignee"), r.FormValue("due_date"),
		formInt64(r, "milestone_id")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, t.Ref+" updated.")
}

func (s *Server) taskStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	t, err := s.St.Task(id)
	if err != nil {
		s.notFound(w)
		return
	}
	if err := s.St.SetTaskStatus(id, r.FormValue("status")); err != nil {
		s.fail(w, r, backTo(r, projURL(t.ProjectID, "board")), err)
		return
	}
	// Drag-and-drop posts via fetch and wants a bare OK; forms send "back".
	if r.FormValue("back") == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.redirect(w, r, backTo(r, projURL(t.ProjectID, "board")), "")
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	t, err := s.St.Task(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(t.ProjectID, "board"))
	if err := s.St.DeleteTask(id); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, t.Ref+" deleted.")
}

// --- Milestones ---

func (s *Server) createMilestone(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "plan"))
	if err := s.St.CreateMilestone(id, r.FormValue("name"), r.FormValue("due_date"), r.FormValue("notes")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Milestone added.")
}

func (s *Server) updateMilestone(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, "/projects")
	if err := s.St.UpdateMilestone(id, r.FormValue("name"), r.FormValue("due_date"), r.FormValue("notes")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Milestone updated.")
}

func (s *Server) toggleMilestone(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.SetMilestoneDone(id, r.FormValue("done") == "1"); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "")
}

func (s *Server) deleteMilestone(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.DeleteMilestone(id); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "Milestone deleted.")
}

// --- RAID ---

func (s *Server) createRaid(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "raid"))
	item, err := s.St.CreateRaidItem(id, r.FormValue("kind"), r.FormValue("title"),
		r.FormValue("detail"), r.FormValue("owner"), r.FormValue("mitigation"),
		r.FormValue("due_date"), formInt(r, "probability"), formInt(r, "impact"))
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, item.Ref+" added.")
}

func (s *Server) updateRaid(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	item, err := s.St.RaidItem(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(item.ProjectID, "raid"))
	if err := s.St.UpdateRaidItem(id, r.FormValue("title"), r.FormValue("detail"),
		r.FormValue("owner"), r.FormValue("mitigation"), r.FormValue("due_date"),
		formInt(r, "probability"), formInt(r, "impact")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, item.Ref+" updated.")
}

func (s *Server) raidStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	item, err := s.St.RaidItem(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(item.ProjectID, "raid"))
	if err := s.St.SetRaidStatus(id, r.FormValue("status")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, item.Ref+" "+r.FormValue("status")+".")
}

func (s *Server) deleteRaid(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	item, err := s.St.RaidItem(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(item.ProjectID, "raid"))
	if err := s.St.DeleteRaidItem(id); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, item.Ref+" deleted.")
}

// --- Decisions ---

func (s *Server) createDecision(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "decisions"))
	d, err := s.St.CreateDecision(id, r.FormValue("title"), r.FormValue("context"))
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, d.Ref+" raised.")
}

func (s *Server) recordDecision(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	d, err := s.St.Decision(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(d.ProjectID, "decisions"))
	if err := s.St.RecordDecision(id, r.FormValue("outcome"), r.FormValue("decided_by")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, d.Ref+" recorded as decided.")
}

func (s *Server) updateDecision(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	d, err := s.St.Decision(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(d.ProjectID, "decisions"))
	if err := s.St.UpdateDecision(id, r.FormValue("title"), r.FormValue("context"),
		r.FormValue("outcome"), r.FormValue("decided_by")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, d.Ref+" updated.")
}

func (s *Server) deleteDecision(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	d, err := s.St.Decision(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(d.ProjectID, "decisions"))
	if err := s.St.DeleteDecision(id); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, d.Ref+" deleted.")
}

// --- Stakeholders & RACI ---

func (s *Server) createStakeholder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "people"))
	if err := s.St.CreateStakeholder(id, r.FormValue("name"), r.FormValue("role"),
		r.FormValue("influence"), r.FormValue("interest"), r.FormValue("attitude"),
		r.FormValue("notes")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Stakeholder added.")
}

func (s *Server) updateStakeholder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, "/projects")
	if err := s.St.UpdateStakeholder(id, r.FormValue("name"), r.FormValue("role"),
		r.FormValue("influence"), r.FormValue("interest"), r.FormValue("attitude"),
		r.FormValue("notes")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Stakeholder updated.")
}

func (s *Server) deleteStakeholder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.DeleteStakeholder(id); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "Stakeholder removed.")
}

func (s *Server) createRaciActivity(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "people"))
	if err := s.St.CreateRaciActivity(id, r.FormValue("name")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Activity added to the RACI.")
}

func (s *Server) setRaci(w http.ResponseWriter, r *http.Request) {
	back := backTo(r, "/projects")
	if err := s.St.SetRaci(formInt64(r, "activity_id"), formInt64(r, "stakeholder_id"),
		strings.ToUpper(strings.TrimSpace(r.FormValue("letter")))); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "")
}

func (s *Server) deleteRaciActivity(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.DeleteRaciActivity(id); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "Activity removed.")
}

// --- Scope, requirements, tests, change requests ---

func (s *Server) createScopeItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "case"))
	item, err := s.St.CreateScopeItem(id, r.FormValue("classification"), r.FormValue("title"),
		r.FormValue("owner"), r.FormValue("rationale"), r.FormValue("acceptance_criteria"), r.FormValue("status"))
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, item.Ref+" added to scope.")
}

func (s *Server) updateScopeItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	item, err := s.St.ScopeItem(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(item.ProjectID, "case"))
	if err := s.St.UpdateScopeItem(id, r.FormValue("classification"), r.FormValue("title"),
		r.FormValue("owner"), r.FormValue("rationale"), r.FormValue("acceptance_criteria"),
		r.FormValue("status")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, item.Ref+" updated.")
}

func (s *Server) deleteScopeItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	item, err := s.St.ScopeItem(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(item.ProjectID, "case"))
	if err := s.St.DeleteScopeItem(id); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, item.Ref+" deleted.")
}

func (s *Server) approveScopeBaseline(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "case"))
	baseline, err := s.St.ApproveScopeBaseline(id, r.FormValue("approved_by"),
		r.FormValue("approved_at"), r.FormValue("notes"))
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Scope baseline v"+strconv.Itoa(baseline.Version)+" approved.")
}

func (s *Server) createRequirement(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "requirements"))
	req, err := s.St.CreateRequirement(id, r.FormValue("title"), r.FormValue("detail"),
		r.FormValue("moscow"), r.FormValue("source"))
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, req.Ref+" added.")
}

func (s *Server) updateRequirement(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	req, err := s.St.Requirement(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(req.ProjectID, "requirements"))
	if err := s.St.UpdateRequirement(id, r.FormValue("title"), r.FormValue("detail"),
		r.FormValue("moscow"), r.FormValue("status"), r.FormValue("source")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, req.Ref+" updated.")
}

func (s *Server) deleteRequirement(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	req, err := s.St.Requirement(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(req.ProjectID, "requirements"))
	if err := s.St.DeleteRequirement(id); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, req.Ref+" deleted.")
}

func (s *Server) createTest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "requirements"))
	if err := s.St.CreateTest(id, formInt64(r, "requirement_id"), r.FormValue("name"),
		r.FormValue("steps"), r.FormValue("expected")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Test added.")
}

func (s *Server) testStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, "/projects")
	if err := s.St.SetTestStatus(id, r.FormValue("status"), r.FormValue("tested_by"),
		r.FormValue("notes")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Test result recorded.")
}

func (s *Server) updateTest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, "/projects")
	if err := s.St.UpdateTest(id, formInt64(r, "requirement_id"), r.FormValue("name"),
		r.FormValue("steps"), r.FormValue("expected")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Test updated.")
}

func (s *Server) deleteTest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.DeleteTest(id); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "Test deleted.")
}

func (s *Server) createChange(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "changes"))
	if err := s.St.CreateChangeRequestWithImpact(id, r.FormValue("title"), r.FormValue("description"),
		r.FormValue("impact"), r.FormValue("raised_by"), formFloat(r, "cost_impact"),
		formInt(r, "schedule_impact_days"), r.FormValue("target_date_impact"),
		formInt64s(r, "scope_item_ids"), formInt64s(r, "requirement_ids")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Change request raised.")
}

func (s *Server) updateChange(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	c, err := s.St.ChangeRequest(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(c.ProjectID, "changes"))
	if err := s.St.UpdateChangeRequest(id, r.FormValue("title"), r.FormValue("description"),
		r.FormValue("impact"), r.FormValue("raised_by"), formFloat(r, "cost_impact"),
		formInt(r, "schedule_impact_days"), r.FormValue("target_date_impact"),
		formInt64s(r, "scope_item_ids"), formInt64s(r, "requirement_ids")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, c.Ref+" updated.")
}

func (s *Server) decideChange(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, "/projects")
	if err := s.St.DecideChangeRequest(id, r.FormValue("status")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Change request "+r.FormValue("status")+".")
}

func (s *Server) deleteChange(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.DeleteChangeRequest(id); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "Change request deleted.")
}

// --- Pain points ---

func (s *Server) createPainPoint(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "discovery"))
	if err := s.St.CreatePainPoint(id, r.FormValue("description"), r.FormValue("process_area"),
		r.FormValue("impact"), r.FormValue("frequency")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Pain point captured.")
}

func (s *Server) deletePainPoint(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.DeletePainPoint(id); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "Pain point removed.")
}

// --- Benefits ---

func (s *Server) createBenefit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "benefits"))
	if err := s.St.CreateBenefit(id, r.FormValue("name"), r.FormValue("category"),
		r.FormValue("unit"), r.FormValue("direction"),
		formFloatPtr(r, "baseline_value"), formFloatPtr(r, "target_value"),
		r.FormValue("baseline_date"), formFloat(r, "annual_value"), r.FormValue("notes")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Benefit defined.")
}

func (s *Server) updateBenefit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	b, err := s.St.Benefit(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(b.ProjectID, "benefits"))
	if err := s.St.UpdateBenefit(id, r.FormValue("name"), r.FormValue("category"),
		r.FormValue("unit"), r.FormValue("direction"),
		formFloatPtr(r, "baseline_value"), formFloatPtr(r, "target_value"),
		r.FormValue("baseline_date"), formFloat(r, "annual_value"), r.FormValue("notes")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, b.Ref+" updated.")
}

func (s *Server) addMeasurement(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	b, err := s.St.Benefit(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(b.ProjectID, "benefits"))
	raw := formFloatPtr(r, "value")
	if raw == nil {
		s.fail(w, r, back, errValidation("Enter the measured value"))
		return
	}
	if err := s.St.AddMeasurement(id, *raw, r.FormValue("measured_at"), r.FormValue("notes")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Measurement recorded for "+b.Ref+".")
}

func (s *Server) deleteMeasurement(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.DeleteMeasurement(id); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "Measurement deleted.")
}

func (s *Server) deleteBenefit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.DeleteBenefit(id); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "Benefit deleted.")
}

// --- Readiness ---

func (s *Server) createReadiness(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "implementation"))
	if err := s.St.CreateReadinessItem(id, r.FormValue("category"), r.FormValue("item"),
		r.FormValue("owner")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Checklist item added.")
}

func (s *Server) seedReadiness(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "implementation"))
	if err := s.St.SeedReadinessChecklist(id); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Standard go-live checklist added. Adapt it to your project.")
}

func (s *Server) toggleReadiness(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.SetReadinessDone(id, r.FormValue("done") == "1"); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "")
}

func (s *Server) deleteReadiness(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.DeleteReadinessItem(id); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "Checklist item removed.")
}

// --- Lessons ---

func (s *Server) createLesson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := backTo(r, projURL(id, "close"))
	if err := s.St.CreateLesson(id, r.FormValue("category"), r.FormValue("lesson"),
		r.FormValue("recommendation")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Lesson recorded.")
}

func (s *Server) deleteLesson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.DeleteLesson(id); err != nil {
		s.fail(w, r, backTo(r, "/projects"), err)
		return
	}
	s.redirect(w, r, backTo(r, "/projects"), "Lesson deleted.")
}

func errValidation(msg string) error {
	return &store.ValidationError{Problems: []string{msg}}
}
