package coach

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
)

func date(daysFromNow int) string {
	return time.Now().AddDate(0, 0, daysFromNow).Format("2006-01-02")
}

func f(v float64) sql.NullFloat64 { return sql.NullFloat64{Float64: v, Valid: true} }

func baseSnapshot(stage string) *store.Snapshot {
	return &store.Snapshot{
		Project: store.Project{
			ID: 1, Code: "DPM-001", Name: "Test project", Stage: stage, Status: "active",
		},
	}
}

func hasAdvice(t *testing.T, items []Advice, substr string) Advice {
	t.Helper()
	for _, a := range items {
		if strings.Contains(a.Message, substr) {
			return a
		}
	}
	t.Fatalf("expected advice containing %q, got %v", substr, messages(items))
	return Advice{}
}

func noAdvice(t *testing.T, items []Advice, substr string) {
	t.Helper()
	for _, a := range items {
		if strings.Contains(a.Message, substr) {
			t.Fatalf("did not expect advice containing %q, got %v", substr, messages(items))
		}
	}
}

func messages(items []Advice) []string {
	var out []string
	for _, a := range items {
		out = append(out, a.Message)
	}
	return out
}

// --- Gate criteria ---

func TestIntakeGateUnmetOnEmptyProject(t *testing.T) {
	g := CheckGate(baseSnapshot("intake"))
	if g.AllMet() {
		t.Fatal("empty intake project should not pass the gate")
	}
	if len(g.Unmet()) != 4 {
		t.Fatalf("expected 4 unmet criteria, got %d: %v", len(g.Unmet()), g.Unmet())
	}
	if g.NextStage != "discovery" {
		t.Fatalf("next stage = %q, want discovery", g.NextStage)
	}
}

func TestIntakeGateMetWhenComplete(t *testing.T) {
	s := baseSnapshot("intake")
	s.Project.ProblemStatement = strings.Repeat("The invoicing process is manual. ", 3)
	s.Project.Goal = "Cut invoicing effort by half"
	s.Project.Sponsor = "Finance Director"
	s.Project.Lead = "PM"
	if g := CheckGate(s); !g.AllMet() {
		t.Fatalf("expected gate met, unmet: %v", g.Unmet())
	}
}

func TestDiscoveryGateRequiresBaseline(t *testing.T) {
	s := baseSnapshot("discovery")
	s.Project.CurrentState = "Documented"
	s.PainPoints = []store.PainPoint{{Description: "slow"}}
	s.Stakeholders = []store.Stakeholder{{Name: "A"}, {Name: "B"}}
	g := CheckGate(s)
	if g.AllMet() {
		t.Fatal("gate should fail without a baseline benefit")
	}
	s.Benefits = []store.Benefit{{Name: "Hours", BaselineValue: f(40)}}
	if g := CheckGate(s); !g.AllMet() {
		t.Fatalf("expected gate met after baseline, unmet: %v", g.Unmet())
	}
}

func TestDefineGateRequiresTargetsOnAllBenefits(t *testing.T) {
	s := baseSnapshot("define")
	s.Project.BusinessCase = "Case"
	s.Project.ScopeIn = "In"
	s.Project.ScopeOut = "Out"
	s.Requirements = []store.Requirement{{Ref: "REQ-001", Moscow: "must"}}
	s.Benefits = []store.Benefit{
		{Name: "A", BaselineValue: f(40), TargetValue: f(10)},
		{Name: "B", BaselineValue: f(5)}, // no target
	}
	if g := CheckGate(s); g.AllMet() {
		t.Fatal("gate should fail while a benefit has no target")
	}
	s.Benefits[1].TargetValue = f(1)
	if g := CheckGate(s); !g.AllMet() {
		t.Fatalf("expected gate met, unmet: %v", g.Unmet())
	}
}

func TestDefineGateAcceptsStructuredApprovedScope(t *testing.T) {
	s := baseSnapshot("define")
	s.Project.BusinessCase = "Case"
	s.ScopeItems = []store.ScopeItem{
		{Classification: "in", Title: "Deliver workflow", Status: "agreed"},
		{Classification: "out", Title: "Replace ERP", Status: "agreed"},
	}
	s.Requirements = []store.Requirement{{Ref: "REQ-001", Moscow: "must"}}
	s.Benefits = []store.Benefit{{Name: "Hours", BaselineValue: f(40), TargetValue: f(10)}}
	if g := CheckGate(s); g.AllMet() {
		t.Fatal("structured scope should require a baseline approval")
	}
	s.ScopeBaselines = []store.ScopeBaseline{{Version: 1, ApprovedBy: "Sponsor"}}
	if g := CheckGate(s); !g.AllMet() {
		t.Fatalf("approved structured scope should pass: %v", g.Unmet())
	}
}

func TestPlanGateFlagsUnmitigatedHighRisk(t *testing.T) {
	s := baseSnapshot("plan")
	s.Project.TargetEnd = date(60)
	s.Milestones = []store.Milestone{{Name: "M1", DueDate: date(30)}}
	s.Tasks = []store.Task{{Title: "a"}, {Title: "b"}, {Title: "c"}}
	s.Raid = []store.RaidItem{{Kind: "risk", Ref: "RISK-001", Status: "open", Probability: 4, Impact: 4}}
	s.Raci = []store.RaciActivity{{Name: "Build"}}
	g := CheckGate(s)
	if g.AllMet() {
		t.Fatal("gate should fail: high risk with no mitigation")
	}
	s.Raid[0].Mitigation = "Do a spike first"
	if g := CheckGate(s); !g.AllMet() {
		t.Fatalf("expected gate met, unmet: %v", g.Unmet())
	}
}

func TestBuildGateRequiresTestedMusts(t *testing.T) {
	s := baseSnapshot("build")
	s.Project.GoLive = date(30)
	s.Requirements = []store.Requirement{
		{Ref: "REQ-001", Moscow: "must", Tests: []store.Test{{Status: "pass"}}},
		{Ref: "REQ-002", Moscow: "must"}, // untested
	}
	g := CheckGate(s)
	if g.AllMet() {
		t.Fatal("gate should fail with an untested must requirement")
	}
	s.Requirements[1].Tests = []store.Test{{Status: "fail"}}
	if g := CheckGate(s); g.AllMet() {
		t.Fatal("gate should fail with a failing must test")
	}
	s.Requirements[1].Tests[0].Status = "pass"
	if g := CheckGate(s); !g.AllMet() {
		t.Fatalf("expected gate met, unmet: %v", g.Unmet())
	}
}

func TestBenefitsGateNeedsMeasurementsLessonsClosure(t *testing.T) {
	s := baseSnapshot("benefits")
	s.Benefits = []store.Benefit{{Name: "A", BaselineValue: f(40), TargetValue: f(10)}}
	g := CheckGate(s)
	if g.AllMet() {
		t.Fatal("gate should fail without measurements")
	}
	s.Benefits[0].Measurements = []store.BenefitMeasurement{{Value: 12, MeasuredAt: date(-1)}}
	s.Lessons = []store.Lesson{{Lesson: "Start training earlier"}}
	s.Project.ClosureSummary = "Delivered and measured"
	if g := CheckGate(s); !g.AllMet() {
		t.Fatalf("expected gate met, unmet: %v", g.Unmet())
	}
	if g := CheckGate(s); g.NextStage != "" {
		t.Fatalf("benefits stage should have no next stage, got %q", g.NextStage)
	}
}

// --- Health ---

func TestHealthGreenOnCleanProject(t *testing.T) {
	h := Assess(baseSnapshot("plan"))
	if h.Status != "green" {
		t.Fatalf("clean project should be green, got %s (%v)", h.Status, h.Reasons)
	}
}

func TestHealthRedOnCompoundingProblems(t *testing.T) {
	s := baseSnapshot("build")
	s.Milestones = []store.Milestone{{Name: "M", DueDate: date(-10)}}
	s.Raid = []store.RaidItem{{Kind: "risk", Ref: "RISK-001", Status: "open", Probability: 4, Impact: 4}}
	s.Tests = []store.Test{{Status: "fail"}}
	h := Assess(s)
	if h.Status != "red" {
		t.Fatalf("expected red, got %s (score %d, %v)", h.Status, h.Score, h.Reasons)
	}
	if len(h.Reasons) < 3 {
		t.Fatalf("expected reasons for each problem, got %v", h.Reasons)
	}
}

func TestHealthAmberOnOverdueTask(t *testing.T) {
	s := baseSnapshot("build")
	s.Tasks = []store.Task{
		{Title: "a", Status: "todo", DueDate: date(-3)},
		{Title: "b", Status: "todo", DueDate: date(-5)},
		{Title: "c", Status: "todo", DueDate: date(-1)},
	}
	h := Assess(s)
	if h.Status != "amber" {
		t.Fatalf("expected amber, got %s (score %d, %v)", h.Status, h.Score, h.Reasons)
	}
}

func TestHealthDoneTasksNeverOverdue(t *testing.T) {
	s := baseSnapshot("build")
	s.Tasks = []store.Task{{Title: "a", Status: "done", DueDate: date(-30)}}
	h := Assess(s)
	if h.Status != "green" {
		t.Fatalf("done tasks must not count as overdue, got %s (%v)", h.Status, h.Reasons)
	}
}

func TestHealthClosedProject(t *testing.T) {
	s := baseSnapshot("benefits")
	s.Project.Status = "closed"
	if h := Assess(s); h.Status != "closed" {
		t.Fatalf("expected closed, got %s", h.Status)
	}
}

// --- Coach advice ---

func TestCoachBaselineBeforeSolution(t *testing.T) {
	s := baseSnapshot("discovery")
	items := Advise(s)
	a := hasAdvice(t, items, "Capture baseline measurements")
	if a.Severity != "act" {
		t.Fatalf("baseline advice should be 'act', got %s", a.Severity)
	}
	s.Benefits = []store.Benefit{{Name: "Hours", BaselineValue: f(40)}}
	noAdvice(t, Advise(s), "Capture baseline measurements")
}

func TestCoachHighRiskNoMitigation(t *testing.T) {
	s := baseSnapshot("plan")
	s.Raid = []store.RaidItem{{Kind: "risk", Ref: "RISK-002", Status: "open", Probability: 5, Impact: 4}}
	hasAdvice(t, Advise(s), "RISK-002 has no mitigation")
	s.Raid[0].Mitigation = "Pilot with one line first"
	noAdvice(t, Advise(s), "RISK-002 has no mitigation")
}

func TestCoachClosedRiskNotFlagged(t *testing.T) {
	s := baseSnapshot("plan")
	s.Raid = []store.RaidItem{{Kind: "risk", Ref: "RISK-003", Status: "closed", Probability: 5, Impact: 5}}
	noAdvice(t, Advise(s), "RISK-003")
}

func TestCoachGoLiveWithoutTrainingPlan(t *testing.T) {
	s := baseSnapshot("build")
	s.Project.GoLive = date(10)
	hasAdvice(t, Advise(s), "no training plan exists")
	s.Readiness = []store.ReadinessItem{{Category: "training", Item: "Train the team"}}
	noAdvice(t, Advise(s), "no training plan exists")
}

func TestCoachUntestedRequirements(t *testing.T) {
	s := baseSnapshot("build")
	s.Project.GoLive = date(60)
	s.Requirements = []store.Requirement{
		{Ref: "REQ-001", Moscow: "must"},
		{Ref: "REQ-002", Moscow: "must"},
		{Ref: "REQ-003", Moscow: "could"},
	}
	hasAdvice(t, Advise(s), "2 must-have requirements have no test linked")
}

func TestCoachDeliveredButUnmeasured(t *testing.T) {
	s := baseSnapshot("benefits")
	s.Benefits = []store.Benefit{{Name: "Hours", BaselineValue: f(40), TargetValue: f(10)}}
	a := hasAdvice(t, Advise(s), "benefits have not been measured")
	if a.Severity != "act" {
		t.Fatalf("unmeasured benefits should be 'act', got %s", a.Severity)
	}
}

func TestCoachSuggestsClosureWhenEverythingDone(t *testing.T) {
	s := baseSnapshot("benefits")
	s.Benefits = []store.Benefit{{
		Name: "Hours", BaselineValue: f(40), TargetValue: f(10),
		Measurements: []store.BenefitMeasurement{{Value: 9, MeasuredAt: date(-2)}},
	}}
	s.Lessons = []store.Lesson{{Lesson: "x"}}
	s.Project.ClosureSummary = "Done"
	hasAdvice(t, Advise(s), "close the project")
}

func TestAdviceSortedBySeverity(t *testing.T) {
	s := baseSnapshot("define")
	s.Tasks = []store.Task{{Title: "late", Status: "todo", DueDate: date(-2)}}
	items := Advise(s)
	last := 0
	for _, a := range items {
		r := severityRank[a.Severity]
		if r < last {
			t.Fatalf("advice not sorted by severity: %v", messages(items))
		}
		last = r
	}
}

func TestCoachSilentOnClosedProject(t *testing.T) {
	s := baseSnapshot("benefits")
	s.Project.Status = "closed"
	s.Tasks = []store.Task{{Title: "late", Status: "todo", DueDate: date(-30)}}
	if items := Advise(s); len(items) != 0 {
		t.Fatalf("closed project should get no advice, got %v", messages(items))
	}
}

// --- Benefit maths (store logic exercised from coach's perspective) ---

func TestBenefitProgressDecrease(t *testing.T) {
	b := store.Benefit{
		Direction: "decrease", BaselineValue: f(40), TargetValue: f(10),
		Measurements: []store.BenefitMeasurement{{Value: 25}},
	}
	pct, ok := b.Progress()
	if !ok || pct != 50 {
		t.Fatalf("expected 50%% progress, got %.1f (ok=%v)", pct, ok)
	}
	if b.Achieved() {
		t.Fatal("25 vs target 10 (decrease) should not be achieved")
	}
	b.Measurements[0].Value = 8
	if !b.Achieved() {
		t.Fatal("8 vs target 10 (decrease) should be achieved")
	}
}

func TestBenefitRealisedAnnualValueCapped(t *testing.T) {
	b := store.Benefit{
		Direction: "decrease", BaselineValue: f(40), TargetValue: f(20), AnnualValue: 12000,
		Measurements: []store.BenefitMeasurement{{Value: 10}}, // beyond target
	}
	if got := b.RealisedAnnualValue(); got != 12000 {
		t.Fatalf("realised value should cap at 100%%: got %.0f", got)
	}
}

func TestNextActionOnReadyGate(t *testing.T) {
	s := baseSnapshot("intake")
	s.Project.ProblemStatement = strings.Repeat("Manual data entry into ERP. ", 3)
	s.Project.Goal = "Automate"
	s.Project.Sponsor = "Ops Director"
	s.Project.Lead = "PM"
	a, ok := NextAction(s)
	if !ok || !strings.Contains(a.Message, "ready to move to Discovery") {
		t.Fatalf("expected gate-ready next action, got %+v ok=%v", a, ok)
	}
}

// --- Momentum & trend rules ---

func timestamp(daysAgo int) string {
	return time.Now().AddDate(0, 0, -daysAgo).Format("2006-01-02 15:04:05")
}

func TestCoachStalledProject(t *testing.T) {
	s := baseSnapshot("plan")
	s.LastActivityAt = timestamp(30)
	hasAdvice(t, Advise(s), "Nothing has been recorded on this project for")
	s.LastActivityAt = timestamp(3)
	noAdvice(t, Advise(s), "Nothing has been recorded")
}

func TestCoachStageDwell(t *testing.T) {
	s := baseSnapshot("intake")
	s.Project.CreatedAt = timestamp(20) // over the 14-day intake limit
	s.LastActivityAt = timestamp(1)
	hasAdvice(t, Advise(s), "has been in Intake for")

	// A recent gate transition resets the clock.
	s2 := baseSnapshot("plan")
	s2.GateHistory = []store.GateEntry{{FromStage: "define", ToStage: "plan", MovedAt: timestamp(5)}}
	noAdvice(t, Advise(s2), "has been in Plan for")
}

func TestCoachOverdueDependency(t *testing.T) {
	s := baseSnapshot("build")
	s.Raid = []store.RaidItem{{Kind: "dependency", Ref: "DEP-001", Status: "open", DueDate: date(-6), Impact: 3}}
	a := hasAdvice(t, Advise(s), "DEP-001 is 6 days past its needed-by date")
	if a.Severity != "act" {
		t.Fatalf("overdue dependency should be 'act', got %s", a.Severity)
	}
	s.Raid[0].Status = "closed"
	noAdvice(t, Advise(s), "DEP-001")
}

func TestCoachBenefitTrendingWrongWay(t *testing.T) {
	s := baseSnapshot("benefits")
	s.Benefits = []store.Benefit{{
		Ref: "BEN-001", Name: "Hours", Direction: "decrease", Unit: "hrs",
		BaselineValue: f(40), TargetValue: f(10),
		Measurements: []store.BenefitMeasurement{
			{Value: 20, MeasuredAt: date(-30)},
			{Value: 28, MeasuredAt: date(-2)}, // got worse
		},
	}}
	hasAdvice(t, Advise(s), "BEN-001 is trending the wrong way")
	// Improving values must not trigger it.
	s.Benefits[0].Measurements[1].Value = 15
	noAdvice(t, Advise(s), "trending the wrong way")
}

func TestCoachStaleMeasurement(t *testing.T) {
	s := baseSnapshot("benefits")
	s.Benefits = []store.Benefit{{
		Ref: "BEN-001", Name: "Hours", Direction: "decrease", Unit: "hrs",
		BaselineValue: f(40), TargetValue: f(10),
		Measurements: []store.BenefitMeasurement{{Value: 20, MeasuredAt: date(-45)}},
	}}
	hasAdvice(t, Advise(s), "BEN-001 has not been measured for")
	// Achieved benefits don't nag for re-measurement.
	s.Benefits[0].Measurements[0].Value = 8
	noAdvice(t, Advise(s), "has not been measured for")
}

func TestCoachResistantPowerfulStakeholder(t *testing.T) {
	s := baseSnapshot("discovery")
	s.Stakeholders = []store.Stakeholder{
		{Name: "Dave", Influence: "high", Attitude: "resistant"},
		{Name: "Jo", Influence: "low", Attitude: "resistant"},
	}
	items := Advise(s)
	hasAdvice(t, items, "Win over Dave")
	noAdvice(t, items, "Win over Jo")
}

func TestCoachTooMuchWIP(t *testing.T) {
	s := baseSnapshot("build")
	for i := 0; i < 6; i++ {
		s.Tasks = append(s.Tasks, store.Task{Title: "t", Status: "doing"})
	}
	hasAdvice(t, Advise(s), "tasks are in progress at once")
}

func TestCoachGateReadyAdvice(t *testing.T) {
	s := baseSnapshot("intake")
	s.Project.ProblemStatement = strings.Repeat("Manual data entry into ERP. ", 3)
	s.Project.Goal = "Automate"
	s.Project.Sponsor = "Ops Director"
	s.Project.Lead = "PM"
	a := hasAdvice(t, Advise(s), "ready to move to Discovery")
	if a.Severity != "consider" {
		t.Fatalf("gate-ready advice should be 'consider', got %s", a.Severity)
	}
}

func TestCoachChangeLoadWarning(t *testing.T) {
	s := baseSnapshot("build")
	s.Project.GoLive = date(60)
	for i := 0; i < 3; i++ {
		s.Changes = append(s.Changes, store.ChangeRequest{Status: "approved"})
	}
	hasAdvice(t, Advise(s), "check the target end date still holds")
}
