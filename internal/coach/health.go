package coach

import (
	"fmt"
	"strings"

	"digipm/internal/store"
)

// Health is the computed RAG status of a project with the reasons behind it.
type Health struct {
	Status  string // "green", "amber", "red" — or "closed"
	Score   int    // accumulated concern points
	Reasons []string
}

// Concern thresholds: a handful of small worries makes a project Amber,
// serious or compounding problems make it Red.
const (
	amberThreshold = 3
	redThreshold   = 7
)

// Assess computes project health from schedule, risks, testing, decisions
// and delivery signals. Closed projects report status "closed".
func Assess(s *store.Snapshot) Health {
	p := s.Project
	if p.IsClosed() {
		return Health{Status: "closed"}
	}
	h := Health{}
	add := func(points int, reason string) {
		h.Score += points
		h.Reasons = append(h.Reasons, reason)
	}

	// Schedule.
	if n := overdueMilestones(s); n > 0 {
		add(3*n, fmt.Sprintf("%s overdue", plural(n, "milestone is", "milestones are")))
	}
	if n := overdueTasks(s); n > 0 {
		pts := n
		if pts > 3 {
			pts = 3
		}
		add(pts, fmt.Sprintf("%s overdue", plural(n, "task is", "tasks are")))
	}
	if d, ok := store.DaysUntil(p.TargetEnd); ok {
		stageIdx := store.StageIndex(p.Stage)
		if d < 0 {
			add(4, fmt.Sprintf("Target end date passed %d days ago and the project is still open", -d))
		} else if d <= 14 && stageIdx < store.StageIndex("implement") {
			add(2, fmt.Sprintf("Only %d days to the target end date but the project is still in %s", d, p.StageName()))
		}
	}

	// Risks and issues.
	if n := unmitigatedHighRisks(s); n > 0 {
		add(3*n, fmt.Sprintf("%s no mitigation", plural(n, "high risk has", "high risks have")))
	}
	for _, r := range s.Raid {
		if r.Kind == "issue" && r.Status == "open" && r.Severity() == "high" {
			add(2, fmt.Sprintf("High-impact issue %s is open", r.Ref))
		}
	}

	// Testing quality signal once the build has started.
	if store.StageIndex(p.Stage) >= store.StageIndex("build") {
		if n := failingTests(s); n > 0 {
			add(3, fmt.Sprintf("%s failing", plural(n, "test is", "tests are")))
		}
	}

	// Delivery flow.
	if n := blockedTasks(s); n >= 2 {
		add(1, fmt.Sprintf("%d tasks are blocked", n))
	}
	if n := pendingChanges(s); n > 0 && store.StageIndex(p.Stage) >= store.StageIndex("build") {
		add(1, fmt.Sprintf("%s awaiting a decision", plural(n, "change request is", "change requests are")))
	}

	// Adoption: delivered but never measured.
	if p.Stage == "benefits" && len(s.Benefits) > 0 && unmeasuredBenefits(s) == len(s.Benefits) {
		add(2, "Delivered, but no benefit has been measured yet")
	}

	switch {
	case h.Score >= redThreshold:
		h.Status = "red"
	case h.Score >= amberThreshold:
		h.Status = "amber"
	default:
		h.Status = "green"
	}
	return h
}

// Summary joins the reasons into one sentence for compact display.
func (h Health) Summary() string {
	if len(h.Reasons) == 0 {
		return "No concerns detected"
	}
	return strings.Join(h.Reasons, ". ")
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, pluralForm)
}
