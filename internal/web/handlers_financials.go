package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/coach"
	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
)

func financialFormValue(r *http.Request, key, label string) (float64, error) {
	raw := strings.TrimSpace(r.FormValue(key))
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 {
		return 0, &store.ValidationError{Problems: []string{label + " must be a non-negative number"}}
	}
	return v, nil
}

func (s *Server) updateFinancials(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := "/projects/" + itoa64(id) + "/case"
	estimated, err := financialFormValue(r, "estimated_cost", "Estimated cost")
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	approved, err := financialFormValue(r, "approved_budget", "Approved budget")
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	actual, err := financialFormValue(r, "actual_cost", "Actual cost")
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	if err := s.St.SaveFinancials(id, estimated, approved, actual, r.FormValue("financial_notes")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Financials saved. ROI and payback have been recalculated from expected benefits.")
}

func (s *Server) captureStatusSnapshot(w http.ResponseWriter, r *http.Request) {
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
	health := coach.Assess(snap)
	finance := store.SummariseFinancials(snap.Financials, snap.Benefits)
	record := store.ProjectStatusSnapshot{
		ProjectID: id, Stage: snap.Project.Stage, ProjectStatus: snap.Project.Status,
		HealthStatus: health.Status, HealthSummary: health.Summary(),
		TotalTasks: len(snap.Tasks), ExpectedAnnualValue: finance.ExpectedAnnualValue,
		RealisedAnnualValue: finance.RealisedAnnualValue,
		EstimatedCost:       snap.Financials.EstimatedCost,
		ApprovedBudget:      snap.Financials.ApprovedBudget, ActualCost: snap.Financials.ActualCost,
		Note: strings.TrimSpace(r.FormValue("note")),
	}
	for _, task := range snap.Tasks {
		if task.Status == "done" {
			record.DoneTasks++
		} else if task.Overdue() {
			record.OverdueTasks++
		}
	}
	for _, item := range snap.Raid {
		if item.Status == "open" {
			record.OpenRaidItems++
		}
	}
	for _, milestone := range snap.Milestones {
		if milestone.Overdue() {
			record.OverdueMilestones++
		}
	}
	created, err := s.St.CreateStatusSnapshot(record)
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, fmt.Sprintf("Status snapshot captured at %s.", created.CapturedAt))
}
