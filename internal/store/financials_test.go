package store

import "testing"

func TestFinancialSummaryAndPersistence(t *testing.T) {
	s := testStore(t)
	p, err := s.CreateProject("Financial case", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	base, target := 100.0, 50.0
	if err := s.CreateBenefit(p.ID, "Reduced handling", "cost_saved", "minutes", "decrease",
		&base, &target, Today(), 12000, ""); err != nil {
		t.Fatal(err)
	}
	benefits, err := s.Benefits(p.ID)
	if err != nil || len(benefits) != 1 {
		t.Fatalf("benefits: %v %+v", err, benefits)
	}
	if err := s.AddMeasurement(benefits[0].ID, 75, Today(), "halfway"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveFinancials(p.ID, 2500, 3000, 0, "Includes licences"); err != nil {
		t.Fatal(err)
	}
	fin, err := s.Financials(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fin.InvestmentCost() != 3000 || fin.Notes != "Includes licences" {
		t.Fatalf("unexpected persisted financials: %+v", fin)
	}
	benefits, _ = s.Benefits(p.ID)
	summary := SummariseFinancials(fin, benefits)
	if summary.ExpectedAnnualValue != 12000 || summary.RealisedAnnualValue != 6000 {
		t.Fatalf("unexpected value summary: %+v", summary)
	}
	if !summary.HasROI || summary.ROI != 300 {
		t.Fatalf("ROI = %v, want 300%%", summary.ROI)
	}
	if !summary.HasPayback || summary.PaybackMonths != 3 {
		t.Fatalf("payback = %v months, want 3", summary.PaybackMonths)
	}
	if err := s.SaveFinancials(p.ID, -1, 0, 0, ""); err == nil {
		t.Fatal("negative cost should be rejected")
	}
}

func TestStatusSnapshotHistoryTracksMovement(t *testing.T) {
	s := testStore(t)
	p, _ := s.CreateProject("Trend", "", "", "", "", "")
	first, err := s.CreateStatusSnapshot(ProjectStatusSnapshot{
		ProjectID: p.ID, Stage: "plan", ProjectStatus: "active", HealthStatus: "red",
		HealthSummary: "Delivery concerns", TotalTasks: 10, DoneTasks: 2,
		OpenRaidItems: 5, OverdueTasks: 3, OverdueMilestones: 1,
		ExpectedAnnualValue: 10000, RealisedAnnualValue: 1000, ApprovedBudget: 4000,
		Note: "First steering review",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == 0 || first.CapturedAt == "" {
		t.Fatalf("snapshot was not returned after insert: %+v", first)
	}
	if _, err := s.CreateStatusSnapshot(ProjectStatusSnapshot{
		ProjectID: p.ID, Stage: "build", ProjectStatus: "active", HealthStatus: "amber",
		HealthSummary: "Recovering", TotalTasks: 10, DoneTasks: 6,
		OpenRaidItems: 3, OverdueTasks: 1, ExpectedAnnualValue: 10000,
		RealisedAnnualValue: 3500, ApprovedBudget: 4000, ActualCost: 3000,
		Note: "Second steering review",
	}); err != nil {
		t.Fatal(err)
	}
	history, err := s.StatusHistory(p.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || !history[0].HasPrevious {
		t.Fatalf("unexpected history: %+v", history)
	}
	latest := history[0]
	if latest.HealthMovement != "improved" || latest.TaskCompletionDelta != 40 {
		t.Fatalf("health/task trend incorrect: %+v", latest)
	}
	if latest.OpenRaidDelta != -2 || latest.OverdueDelta != -3 || latest.RealisedValueDelta != 2500 {
		t.Fatalf("indicator deltas incorrect: %+v", latest)
	}
	if history[1].HasPrevious {
		t.Fatal("oldest record must not claim a prior baseline")
	}
}
