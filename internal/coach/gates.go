// Package coach analyses a project snapshot and produces gate checks,
// health assessments and recommendations. All functions are pure: they
// read the snapshot and never touch the database.
package coach

import (
	"strings"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
)

// Criterion is one recommended completion item for a stage gate.
type Criterion struct {
	Text string // what should be complete
	Why  string // why it matters, shown as guidance
	Met  bool
	Link string // workspace tab that addresses it
}

// GateCheck is the readiness assessment for leaving the current stage.
type GateCheck struct {
	Stage     string
	NextStage string // empty when the next step is closing the project
	Criteria  []Criterion
}

// AllMet reports whether every criterion is satisfied.
func (g GateCheck) AllMet() bool {
	for _, c := range g.Criteria {
		if !c.Met {
			return false
		}
	}
	return true
}

// Unmet returns the text of unsatisfied criteria.
func (g GateCheck) Unmet() []string {
	var out []string
	for _, c := range g.Criteria {
		if !c.Met {
			out = append(out, c.Text)
		}
	}
	return out
}

// MetCount returns how many criteria are satisfied.
func (g GateCheck) MetCount() int {
	n := 0
	for _, c := range g.Criteria {
		if c.Met {
			n++
		}
	}
	return n
}

func filled(s string) bool { return strings.TrimSpace(s) != "" }

// CheckGate evaluates the recommended completion criteria for the
// project's current stage.
func CheckGate(s *store.Snapshot) GateCheck {
	p := s.Project
	g := GateCheck{Stage: p.Stage, NextStage: store.NextStage(p.Stage)}
	add := func(text, why, link string, met bool) {
		g.Criteria = append(g.Criteria, Criterion{Text: text, Why: why, Link: link, Met: met})
	}

	switch p.Stage {
	case "intake":
		add("Problem statement written",
			"A clear problem stops you building a solution in search of a problem. Describe what hurts, who it hurts, and how often.",
			"overview", len(strings.TrimSpace(p.ProblemStatement)) >= 30)
		add("Goal / desired outcome stated",
			"State what 'better' looks like in one or two sentences, before anyone talks about tools.",
			"overview", filled(p.Goal))
		add("Sponsor identified",
			"Someone with authority must want this fixed. Without a sponsor, projects stall the first time priorities clash.",
			"overview", filled(p.Sponsor))
		add("Project lead assigned",
			"One named person owns driving the project forward.",
			"overview", filled(p.Lead))

	case "discovery":
		add("Current process documented",
			"Understand how the work is actually done today — including the workarounds — before designing anything.",
			"discovery", filled(p.CurrentState))
		add("Pain points captured",
			"Specific, concrete pain points justify the project and become your requirements later.",
			"discovery", len(s.PainPoints) >= 1)
		add("Stakeholders identified (at least 2)",
			"Map who is affected, who can block you, and who has to change how they work.",
			"people", len(s.Stakeholders) >= 2)
		add("Baseline measurement captured for at least one benefit",
			"You cannot prove improvement without a before picture. Measure today's performance now — it is impossible to reconstruct later.",
			"benefits", countBenefitsWithBaseline(s) >= 1)

	case "define":
		add("Business case written",
			"Summarise the cost of the problem, the proposed solution, and the expected return. This is what the sponsor approves.",
			"case", filled(p.BusinessCase))
		add("Scope and out-of-scope recorded",
			"Writing down what you will NOT do is the cheapest way to prevent scope creep.",
			"case", scopeRecorded(s))
		add("Structured scope baseline approved",
			"Approval freezes a versioned scope boundary so later changes can be assessed against an agreed starting point.",
			"case", len(s.ScopeItems) == 0 || len(s.ScopeBaselines) > 0)
		add("Requirements captured and prioritised",
			"Requirements turn vague hopes into things you can build and test. Use MoSCoW so everyone knows what is essential.",
			"requirements", len(s.Requirements) >= 1)
		add("Every benefit has a baseline and a target",
			"Baseline → target is the promise the project makes. Targets set now keep everyone honest at the end.",
			"benefits", len(s.Benefits) >= 1 && allBenefitsTargeted(s))

	case "plan":
		add("Target end date set",
			"A date focuses effort. It can move — via a recorded change — but it must exist.",
			"overview", filled(p.TargetEnd))
		add("Milestones with due dates created",
			"Milestones let you see slippage early, while you can still act on it.",
			"plan", countMilestonesWithDates(s) >= 1)
		add("Delivery tasks created (at least 3)",
			"Break the work down until each piece is small enough to finish in days, not weeks.",
			"board", len(s.Tasks) >= 3)
		add("Risks reviewed and recorded",
			"Every project has risks. An empty risk register means nobody has looked.",
			"raid", countRaid(s, "risk") >= 1)
		add("Every high risk has a mitigation",
			"A high risk without a plan is just waiting to become an issue.",
			"raid", unmitigatedHighRisks(s) == 0)
		add("RACI agreed for key activities",
			"When nobody is Accountable, everybody assumes someone else is doing it.",
			"people", len(s.Raci) >= 1)

	case "build":
		add("Every must-have requirement has a test",
			"Untested requirements are unverified promises. Link at least one test to each must-have.",
			"requirements", untestedMusts(s) == 0)
		add("All tests on must-have requirements pass",
			"Go-live with failing must-have tests means shipping known defects into the business.",
			"requirements", failingOrPendingMustTests(s) == 0)
		add("No undecided change requests",
			"Undecided changes create scope ambiguity. Approve or reject them before implementing.",
			"changes", pendingChanges(s) == 0)
		add("Go-live date set",
			"Implementation needs a date so training and communications can be planned backwards from it.",
			"implementation", filled(p.GoLive))

	case "implement":
		add("Readiness checklist created",
			"Go-live fails on the human side more often than the technical side. Work the checklist.",
			"implementation", len(s.Readiness) >= 1)
		add("All training items complete",
			"If people are not trained, they will keep using the old process and the benefits never appear.",
			"implementation", incompleteReadiness(s, "training") == 0)
		add("All readiness items complete",
			"Everything on the checklist exists because skipping it has burned a project before.",
			"implementation", incompleteReadiness(s, "") == 0)
		add("Go-live date reached",
			"The project moves to Benefits & Close once the solution is live in the business.",
			"implementation", dateReached(p.GoLive))

	case "benefits":
		// The final gate closes the project rather than advancing a stage.
		add("Every benefit has an actual measurement",
			"This is the whole point: measure the after picture and compare it with the baseline.",
			"benefits", unmeasuredBenefits(s) == 0 && len(s.Benefits) > 0)
		add("Lessons learned recorded",
			"Ten minutes of honest reflection makes the next project cheaper and calmer.",
			"close", len(s.Lessons) >= 1)
		add("Closure summary written",
			"Summarise what was delivered and what was measured, so the record outlives memories.",
			"close", filled(p.ClosureSummary))
	}
	return g
}

func scopeRecorded(s *store.Snapshot) bool {
	hasIn, hasOut := filled(s.Project.ScopeIn), filled(s.Project.ScopeOut)
	for _, item := range s.ScopeItems {
		if item.Status == "removed" {
			continue
		}
		if item.Classification == "in" {
			hasIn = true
		} else if item.Classification == "out" {
			hasOut = true
		}
	}
	return hasIn && hasOut
}

// --- snapshot helpers shared with health and advice ---

func countBenefitsWithBaseline(s *store.Snapshot) int {
	n := 0
	for _, b := range s.Benefits {
		if b.HasBaseline() {
			n++
		}
	}
	return n
}

func allBenefitsTargeted(s *store.Snapshot) bool {
	for _, b := range s.Benefits {
		if !b.HasBaseline() || !b.HasTarget() {
			return false
		}
	}
	return true
}

func countMilestonesWithDates(s *store.Snapshot) int {
	n := 0
	for _, m := range s.Milestones {
		if m.DueDate != "" {
			n++
		}
	}
	return n
}

func countRaid(s *store.Snapshot, kind string) int {
	n := 0
	for _, r := range s.Raid {
		if r.Kind == kind {
			n++
		}
	}
	return n
}

func unmitigatedHighRisks(s *store.Snapshot) int {
	n := 0
	for _, r := range s.Raid {
		if r.Kind == "risk" && r.Status == "open" && r.Severity() == "high" && strings.TrimSpace(r.Mitigation) == "" {
			n++
		}
	}
	return n
}

func untestedMusts(s *store.Snapshot) int {
	n := 0
	for _, r := range s.Requirements {
		if r.Moscow == "must" && r.Status != "dropped" && len(r.Tests) == 0 {
			n++
		}
	}
	return n
}

func failingOrPendingMustTests(s *store.Snapshot) int {
	n := 0
	for _, r := range s.Requirements {
		if r.Moscow != "must" || r.Status == "dropped" {
			continue
		}
		for _, t := range r.Tests {
			if t.Status != "pass" {
				n++
			}
		}
	}
	return n
}

func failingTests(s *store.Snapshot) int {
	n := 0
	for _, t := range s.Tests {
		if t.Status == "fail" {
			n++
		}
	}
	return n
}

func pendingChanges(s *store.Snapshot) int {
	n := 0
	for _, c := range s.Changes {
		if c.Status == "proposed" {
			n++
		}
	}
	return n
}

// incompleteReadiness counts unfinished readiness items, optionally
// filtered by category ("" = all).
func incompleteReadiness(s *store.Snapshot, category string) int {
	n := 0
	for _, r := range s.Readiness {
		if category != "" && r.Category != category {
			continue
		}
		if !r.Done {
			n++
		}
	}
	return n
}

func unmeasuredBenefits(s *store.Snapshot) int {
	n := 0
	for _, b := range s.Benefits {
		if len(b.Measurements) == 0 {
			n++
		}
	}
	return n
}

// dateReached reports whether an ISO date is set and today or earlier.
func dateReached(date string) bool {
	d, ok := store.DaysUntil(date)
	return ok && d <= 0
}

func overdueTasks(s *store.Snapshot) int {
	n := 0
	for _, t := range s.Tasks {
		if t.Overdue() {
			n++
		}
	}
	return n
}

func overdueMilestones(s *store.Snapshot) int {
	n := 0
	for _, m := range s.Milestones {
		if m.Overdue() {
			n++
		}
	}
	return n
}

func blockedTasks(s *store.Snapshot) int {
	n := 0
	for _, t := range s.Tasks {
		if t.Status == "blocked" {
			n++
		}
	}
	return n
}
