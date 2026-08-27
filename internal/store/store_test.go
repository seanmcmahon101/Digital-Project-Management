package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/db"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	sqldb, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	return New(sqldb)
}

func TestMigrationsApplyCleanly(t *testing.T) {
	s := testStore(t)
	// Re-running migrations on an up-to-date database must be a no-op.
	if err := db.Migrate(s.DB); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
}

func TestNextRefSequence(t *testing.T) {
	s := testStore(t)
	for i, want := range []string{"REQ-001", "REQ-002", "REQ-003"} {
		got, err := s.NextRef(1, "REQ")
		if err != nil {
			t.Fatalf("NextRef #%d: %v", i, err)
		}
		if got != want {
			t.Fatalf("NextRef #%d = %q, want %q", i, got, want)
		}
	}
	// Independent per project and per kind.
	if got, _ := s.NextRef(2, "REQ"); got != "REQ-001" {
		t.Fatalf("project 2 should start fresh, got %q", got)
	}
	if got, _ := s.NextRef(1, "RISK"); got != "RISK-001" {
		t.Fatalf("different kind should start fresh, got %q", got)
	}
}

func TestProjectLifecycle(t *testing.T) {
	s := testStore(t)
	p, err := s.CreateProject("Shop floor data capture", "Ops Director", "Sam", "Operations",
		"Paper travellers are re-keyed into the ERP", "Digitise capture at source")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Code != "DPM-001" || p.Stage != "intake" {
		t.Fatalf("unexpected new project: %+v", p)
	}

	// Advancing with unmet criteria requires a reason.
	if _, err := s.AdvanceStage(p.ID, []string{"Problem statement"}, ""); err == nil {
		t.Fatal("override without a reason should fail")
	}
	p2, err := s.AdvanceStage(p.ID, []string{"Problem statement"}, "Sponsor wants to fast-track")
	if err != nil {
		t.Fatalf("override advance: %v", err)
	}
	if p2.Stage != "discovery" {
		t.Fatalf("stage = %q, want discovery", p2.Stage)
	}
	hist, err := s.GateHistory(p.ID)
	if err != nil || len(hist) != 1 {
		t.Fatalf("gate history: %v (%d entries)", err, len(hist))
	}
	if !hist[0].Overridden || hist[0].OverrideReason == "" || hist[0].UnmetCriteria == "" {
		t.Fatalf("override not recorded: %+v", hist[0])
	}

	if err := s.CloseProject(p.ID, "Cancelled at gate review", true); err != nil {
		t.Fatalf("close: %v", err)
	}
	p3, _ := s.Project(p.ID)
	if p3.Status != "cancelled" || !p3.IsClosed() {
		t.Fatalf("expected cancelled, got %+v", p3.Status)
	}
}

func TestProjectHoldAndResume(t *testing.T) {
	s := testStore(t)
	p, err := s.CreateProject("Paused delivery", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetProjectHold(p.ID, true, ""); err == nil {
		t.Fatal("putting a project on hold without a reason should fail")
	}
	if err := s.SetProjectHold(p.ID, true, "Waiting for procurement approval"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdvanceStage(p.ID, nil, ""); err == nil {
		t.Fatal("on-hold project was allowed to advance")
	}
	p, err = s.Project(p.ID)
	if err != nil || p.Status != "on_hold" {
		t.Fatalf("held project = %+v, err=%v", p, err)
	}
	if err := s.SetProjectHold(p.ID, false, ""); err != nil {
		t.Fatal(err)
	}
	p, err = s.Project(p.ID)
	if err != nil || p.Status != "active" {
		t.Fatalf("resumed project = %+v, err=%v", p, err)
	}
}

func TestAdvanceStageRollsBackWhenHistoryCannotBeRecorded(t *testing.T) {
	s := testStore(t)
	p, err := s.CreateProject("Transactional gate", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`CREATE TRIGGER reject_gate_history BEFORE INSERT ON gate_history
		BEGIN SELECT RAISE(ABORT, 'history unavailable'); END`); err != nil {
		t.Fatal(err)
	}

	if _, err := s.AdvanceStage(p.ID, nil, ""); err == nil {
		t.Fatal("expected gate history failure")
	}
	got, err := s.Project(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stage != "intake" {
		t.Fatalf("stage changed despite failed history insert: %q", got.Stage)
	}
	history, err := s.GateHistory(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("unexpected gate history after rollback: %+v", history)
	}
}

func TestTaskStatusStampsCompletion(t *testing.T) {
	s := testStore(t)
	p, _ := s.CreateProject("P", "", "", "", "", "")
	task, err := s.CreateTask(p.ID, "Do the thing", "", "high", "Sam", "", 0)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.Ref != "TASK-001" {
		t.Fatalf("ref = %q", task.Ref)
	}
	if err := s.SetTaskStatus(task.ID, "done"); err != nil {
		t.Fatalf("set status: %v", err)
	}
	task, _ = s.Task(task.ID)
	if task.CompletedAt == "" {
		t.Fatal("done task should have completed_at stamped")
	}
	if err := s.SetTaskStatus(task.ID, "todo"); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	task, _ = s.Task(task.ID)
	if task.CompletedAt != "" {
		t.Fatal("reopened task should clear completed_at")
	}
	if err := s.SetTaskStatus(task.ID, "bogus"); err == nil {
		t.Fatal("invalid status should be rejected")
	}
}

func TestValidationRejectsBadInput(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreateProject("   ", "", "", "", "", ""); err == nil {
		t.Fatal("blank project name should be rejected")
	}
	p, _ := s.CreateProject("P", "", "", "", "", "")
	if _, err := s.CreateTask(p.ID, "T", "", "medium", "", "not-a-date", 0); err == nil {
		t.Fatal("invalid date should be rejected")
	}
	if err := s.ScoreIdea(1, 9, 0, 0, 0, 0); err == nil {
		t.Fatal("out-of-range idea score should be rejected")
	}
}

func TestIdeaConversion(t *testing.T) {
	s := testStore(t)
	id, err := s.CreateIdea("Automate goods-in booking", "Bookings are typed twice", "Stores manager")
	if err != nil {
		t.Fatalf("create idea: %v", err)
	}
	if err := s.ScoreIdea(id, 4, 3, 5, 2, 1); err != nil {
		t.Fatalf("score: %v", err)
	}
	idea, _ := s.Idea(id)
	if idea.Status != "scored" || idea.Score() != 12+6+15-4-1 {
		t.Fatalf("unexpected idea after scoring: %+v score=%d", idea.Status, idea.Score())
	}
	p, err := s.ConvertIdea(id, "Ops Director", "Sam")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	idea, _ = s.Idea(id)
	if idea.Status != "converted" || !idea.ProjectID.Valid || idea.ProjectID.Int64 != p.ID {
		t.Fatalf("idea not linked to project: %+v", idea)
	}
	if _, err := s.ConvertIdea(id, "", ""); err == nil {
		t.Fatal("double conversion should be rejected")
	}
}

func TestBenefitMeasurementFlow(t *testing.T) {
	s := testStore(t)
	p, _ := s.CreateProject("P", "", "", "", "", "")
	base, target := 40.0, 10.0
	if err := s.CreateBenefit(p.ID, "Manual hours", "hours_saved", "hrs/month", "decrease",
		&base, &target, Today(), 18000, ""); err != nil {
		t.Fatalf("create benefit: %v", err)
	}
	benefits, _ := s.Benefits(p.ID)
	if len(benefits) != 1 || benefits[0].Ref != "BEN-001" {
		t.Fatalf("unexpected benefits: %+v", benefits)
	}
	if err := s.AddMeasurement(benefits[0].ID, 25, "", "first month after go-live"); err != nil {
		t.Fatalf("measure: %v", err)
	}
	b, _ := s.Benefit(benefits[0].ID)
	if len(b.Measurements) != 1 {
		t.Fatalf("measurement not stored")
	}
	if got := b.MonthlyHoursSaved(); got != 15 {
		t.Fatalf("hours saved = %v, want 15", got)
	}
	pct, ok := b.Progress()
	if !ok || pct != 50 {
		t.Fatalf("progress = %v ok=%v, want 50", pct, ok)
	}
}

func TestBackupCreatesRestorableCopy(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreateProject("Backed up", "", "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path, err := s.Backup(dir)
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Size() == 0 {
		t.Fatalf("backup file missing/empty: %v", err)
	}
	// The backup must open as a valid database containing the data.
	bdb, err := db.Open(path)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer bdb.Close()
	var name string
	if err := bdb.QueryRow(`SELECT name FROM projects`).Scan(&name); err != nil || name != "Backed up" {
		t.Fatalf("backup content: %q err=%v", name, err)
	}
}

func TestAutoBackupOncePerDayAndPrunes(t *testing.T) {
	s := testStore(t)
	dir := t.TempDir()
	p1, err := s.AutoBackup(dir, 5)
	if err != nil || p1 == "" {
		t.Fatalf("first auto-backup: %v %q", err, p1)
	}
	p2, err := s.AutoBackup(dir, 5)
	if err != nil || p2 != "" {
		t.Fatalf("second same-day auto-backup should be skipped, got %q err=%v", p2, err)
	}
	if got := len(ListBackups(dir)); got != 1 {
		t.Fatalf("expected 1 backup, got %d", got)
	}
}

func TestSeedReadinessChecklistIdempotent(t *testing.T) {
	s := testStore(t)
	p, _ := s.CreateProject("P", "", "", "", "", "")
	if err := s.SeedReadinessChecklist(p.ID); err != nil {
		t.Fatal(err)
	}
	items, _ := s.ReadinessItems(p.ID)
	first := len(items)
	if first == 0 {
		t.Fatal("expected seeded items")
	}
	if err := s.SeedReadinessChecklist(p.ID); err != nil {
		t.Fatal(err)
	}
	items, _ = s.ReadinessItems(p.ID)
	if len(items) != first {
		t.Fatalf("seeding twice duplicated items: %d -> %d", first, len(items))
	}
}

func TestStructuredScopeBaselineAndChangeTraceability(t *testing.T) {
	s := testStore(t)
	p, err := s.CreateProject("Governed scope", "Sponsor", "PM", "Ops", "", "")
	if err != nil {
		t.Fatal(err)
	}
	in, err := s.CreateScopeItem(p.ID, "in", "Automate order entry", "Process owner",
		"Removes duplicate entry", "Orders enter ERP without re-keying", "agreed")
	if err != nil {
		t.Fatalf("create in-scope item: %v", err)
	}
	out, err := s.CreateScopeItem(p.ID, "out", "Replace the ERP", "IT Director",
		"Not justified by this project", "", "agreed")
	if err != nil {
		t.Fatalf("create out-of-scope item: %v", err)
	}
	if in.Ref != "SCP-001" || out.Ref != "SCP-002" {
		t.Fatalf("unexpected scope refs: %s %s", in.Ref, out.Ref)
	}
	b1, err := s.ApproveScopeBaseline(p.ID, "Sponsor", Today(), "Approved at steering group")
	if err != nil || b1.Version != 1 || b1.ScopeSnapshot == "" {
		t.Fatalf("approve baseline 1: %+v err=%v", b1, err)
	}
	b2, err := s.ApproveScopeBaseline(p.ID, "Sponsor", Today(), "Reconfirmed")
	if err != nil || b2.Version != 2 {
		t.Fatalf("approve baseline 2: %+v err=%v", b2, err)
	}
	req, err := s.CreateRequirement(p.ID, "Capture order", "", "must", "Workshop")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateChangeRequestWithImpact(p.ID, "Add approval", "New manager approval", "Extra workflow",
		"Operations", 2500, 5, "2030-06-30", []int64{in.ID}, []int64{req.ID}); err != nil {
		t.Fatalf("create traced change: %v", err)
	}
	changes, err := s.ChangeRequests(p.ID)
	if err != nil || len(changes) != 1 {
		t.Fatalf("load changes: %v %+v", err, changes)
	}
	c := changes[0]
	if c.CostImpact != 2500 || c.ScheduleImpactDays != 5 || !c.AffectsScope(in.ID) || !c.AffectsRequirement(req.ID) {
		t.Fatalf("change impact/links not retained: %+v", c)
	}
	if err := s.UpdateChangeRequest(c.ID, c.Title, c.Description, "Reassessed", c.RaisedBy,
		3000, 7, "2030-07-02", []int64{out.ID}, nil); err != nil {
		t.Fatalf("update change: %v", err)
	}
	c, err = s.ChangeRequest(c.ID)
	if err != nil || c.CostImpact != 3000 || !c.AffectsScope(out.ID) || c.AffectsScope(in.ID) || len(c.Requirements) != 0 {
		t.Fatalf("updated links incorrect: %+v err=%v", c, err)
	}
}

func TestScopeAndChangeValidation(t *testing.T) {
	s := testStore(t)
	p1, _ := s.CreateProject("One", "", "", "", "", "")
	p2, _ := s.CreateProject("Two", "", "", "", "", "")
	if _, err := s.CreateScopeItem(p1.ID, "maybe", "Boundary", "", "", "", "proposed"); err == nil {
		t.Fatal("invalid scope classification should fail")
	}
	foreign, _ := s.CreateScopeItem(p2.ID, "in", "Foreign", "", "", "", "proposed")
	if err := s.CreateChangeRequestWithImpact(p1.ID, "Change", "", "", "", 0, 0, "",
		[]int64{foreign.ID}, nil); err == nil {
		t.Fatal("cross-project scope link should fail")
	}
	if _, err := s.ApproveScopeBaseline(p1.ID, "Sponsor", Today(), ""); err == nil {
		t.Fatal("empty scope baseline should fail")
	}
}
