package coach

import (
	"fmt"
	"strings"

	"digipm/internal/store"
)

// Advice is one recommendation from the Project Coach.
type Advice struct {
	Severity string // "act" (do this now), "soon", "consider"
	Message  string // what to do
	Why      string // why it matters
	Link     string // workspace tab that addresses it
}

var severityRank = map[string]int{"act": 0, "soon": 1, "consider": 2}

// Advise runs every coaching rule against the snapshot and returns
// recommendations, most urgent first.
func Advise(s *store.Snapshot) []Advice {
	p := s.Project
	if p.IsClosed() {
		return nil
	}
	var out []Advice
	add := func(severity, message, why, link string) {
		out = append(out, Advice{Severity: severity, Message: message, Why: why, Link: link})
	}

	// --- Universal delivery hygiene ---
	if n := overdueMilestones(s); n > 0 {
		add("act", fmt.Sprintf("%s overdue — replan or mark complete", plural(n, "milestone is", "milestones are")),
			"Slipped milestones hide schedule problems. Move the date deliberately (and note why) rather than letting it drift.", "plan")
	}
	if n := overdueTasks(s); n > 0 {
		add("act", fmt.Sprintf("%s past the due date", plural(n, "task is", "tasks are")),
			"Overdue tasks either need doing, rescheduling, or deleting. A board full of stale dates stops being trustworthy.", "board")
	}
	for _, r := range s.Raid {
		if r.Kind == "risk" && r.Status == "open" && r.Severity() == "high" && strings.TrimSpace(r.Mitigation) == "" {
			add("act", fmt.Sprintf("High risk %s has no mitigation", r.Ref),
				"A high risk without a plan is just waiting to become an issue. Decide how to reduce, avoid, transfer or accept it.", "raid")
		}
		if r.Kind == "issue" && r.Status == "open" && r.Severity() == "high" {
			add("soon", fmt.Sprintf("High-impact issue %s is open — drive it to resolution", r.Ref),
				"Issues are risks that already happened. The longer they stay open, the more they cost.", "raid")
		}
	}
	for _, d := range s.Decisions {
		if d.Status == "pending" {
			if days, ok := ageInDays(d.CreatedAt); ok && days >= 14 {
				add("soon", fmt.Sprintf("Decision %s has been pending for %d days", d.Ref, days),
					"Undecided questions block work quietly. Put it in front of whoever can decide, with a recommendation.", "decisions")
			}
		}
	}
	if n := blockedTasks(s); n > 0 {
		add("soon", fmt.Sprintf("%s blocked", plural(n, "task is", "tasks are")),
			"Blocked work rarely unblocks itself. Chase the blocker or raise it as an issue.", "board")
	}
	if d, ok := store.DaysUntil(p.TargetEnd); ok && d < 0 {
		add("act", "The target end date has passed but the project is still open",
			"Either replan with a realistic date, or descope. An expired plan hides the true position from everyone.", "overview")
	}

	// --- Stage-specific coaching ---
	switch p.Stage {
	case "intake":
		if len(strings.TrimSpace(p.ProblemStatement)) < 30 {
			add("act", "Write a clear problem statement",
				"Describe what hurts, who it hurts, and how often. If you can't state the problem, you can't judge any solution.", "overview")
		}
		if !filled(p.Sponsor) {
			add("act", "Identify a sponsor",
				"Someone with authority must want this fixed. Without one, the project stalls the first time priorities clash.", "overview")
		}
		if !filled(p.Goal) {
			add("soon", "State the desired outcome",
				"Say what 'better' looks like before anyone talks about tools or systems.", "overview")
		}

	case "discovery":
		if countBenefitsWithBaseline(s) == 0 {
			add("act", "Capture baseline measurements before designing the solution",
				"You cannot prove improvement without a before picture. Measure today's performance now — it is impossible to reconstruct later.", "benefits")
		}
		if !filled(p.CurrentState) {
			add("soon", "Document how the process works today",
				"Walk through the real process with the people who do it — including the workarounds and spreadsheets nobody admits to.", "discovery")
		}
		if len(s.PainPoints) == 0 {
			add("soon", "Capture specific pain points",
				"Concrete pain points ('re-keying takes 2 hours per order') justify the project and become requirements later.", "discovery")
		}
		if len(s.Stakeholders) < 2 {
			add("soon", "Identify the stakeholders",
				"Map who is affected, who can block you, and who has to change how they work. Surprises here are expensive later.", "people")
		}

	case "define":
		if !filled(p.BusinessCase) {
			add("act", "Write the business case",
				"Summarise the cost of the problem, the proposed solution and the expected return. This is what the sponsor approves.", "case")
		}
		if len(s.Requirements) == 0 {
			add("act", "Capture requirements",
				"Requirements turn hopes into things you can build and test. Start from the pain points you found in Discovery.", "requirements")
		}
		if len(s.Benefits) == 0 {
			add("act", "Define the expected benefits",
				"If the project can't name a measurable benefit, it isn't ready to be funded.", "benefits")
		} else if !allBenefitsTargeted(s) {
			add("soon", "Set a baseline and target for every benefit",
				"Baseline → target is the promise the project makes. Set targets now, while nobody is defensive about results.", "benefits")
		}
		if !hasOutOfScope(s) {
			add("consider", "Write down what is out of scope",
				"Out-of-scope lines are the cheapest way to prevent scope creep — far cheaper than arguing later.", "case")
		}
		if len(s.ScopeItems) > 0 && len(s.ScopeBaselines) == 0 {
			add("soon", "Ask the sponsor to approve the scope baseline",
				"A versioned approval makes the agreed boundary explicit and gives later change requests a reliable reference point.", "case")
		}
		if mustShare := mustRequirementShare(s); mustShare > 0.8 && len(s.Requirements) >= 5 {
			add("consider", "Most requirements are marked 'must have' — re-prioritise honestly",
				"When everything is a must, nothing is. Real prioritisation is what lets you deliver the important part sooner.", "requirements")
		}

	case "plan":
		if countMilestonesWithDates(s) == 0 {
			add("act", "Create milestones with dates",
				"Milestones let you see slippage early, while you can still do something about it.", "plan")
		}
		if len(s.Tasks) < 3 {
			add("act", "Break the delivery down into tasks",
				"Break work down until each piece takes days, not weeks. Big vague tasks are where schedules go to die.", "board")
		}
		if countRaid(s, "risk") == 0 {
			add("act", "Record the project's risks",
				"Every project has risks. An empty register doesn't mean you're safe — it means nobody has looked.", "raid")
		}
		if len(s.Raci) == 0 {
			add("soon", "Agree a RACI for the key activities",
				"When nobody is Accountable, everybody assumes someone else is doing it.", "people")
		}
		if !filled(p.TargetEnd) {
			add("soon", "Set a target end date",
				"A date focuses effort. It can move — via a recorded decision — but it must exist.", "overview")
		}

	case "build":
		if n := untestedMusts(s); n > 0 {
			add("act", fmt.Sprintf("%s no test linked", plural(n, "must-have requirement has", "must-have requirements have")),
				"Untested requirements are unverified promises. Write a test that would prove each one works.", "requirements")
		}
		if n := failingTests(s); n > 0 {
			add("act", fmt.Sprintf("%s failing — fix and retest", plural(n, "test is", "tests are")),
				"Going live with known failures means shipping defects into the business and losing user trust on day one.", "requirements")
		}
		if n := pendingChanges(s); n > 0 {
			add("soon", fmt.Sprintf("%s awaiting a decision", plural(n, "change request is", "change requests are")),
				"Undecided changes create scope ambiguity. Approve or reject them so the team builds the right thing.", "changes")
		}
		coachGoLivePrep(s, add)

	case "implement":
		if len(s.Readiness) == 0 {
			add("act", "Create the implementation readiness checklist",
				"Go-live fails on the human side more often than the technical side. Use the standard checklist as a starting point.", "implementation")
		} else {
			if n := incompleteReadiness(s, "training"); n > 0 {
				add("act", fmt.Sprintf("%s outstanding", plural(n, "training item is", "training items are")),
					"If people are not trained, they will keep using the old process and the benefits never appear.", "implementation")
			}
			if n := incompleteReadiness(s, ""); n > 0 {
				add("soon", fmt.Sprintf("%d readiness items still to complete before go-live", n),
					"Each item exists because skipping it has burned a project before.", "implementation")
			}
		}
		if !filled(p.GoLive) {
			add("soon", "Set the go-live date",
				"Training and communications are planned backwards from the go-live date.", "implementation")
		}

	case "benefits":
		if len(s.Benefits) > 0 && unmeasuredBenefits(s) == len(s.Benefits) {
			add("act", "The project has been delivered but benefits have not been measured",
				"This is the whole point of the project. Measure the after picture and compare it with the baseline.", "benefits")
		} else if n := unmeasuredBenefits(s); n > 0 {
			add("soon", fmt.Sprintf("%s no actual measurement yet", plural(n, "benefit has", "benefits have")),
				"Measure every benefit you promised — including the ones that may not have materialised. Honest numbers build credibility.", "benefits")
		}
		if len(s.Lessons) == 0 {
			add("soon", "Record lessons learned",
				"Ten minutes of honest reflection makes the next project cheaper and calmer.", "close")
		}
		gate := CheckGate(s)
		if gate.AllMet() {
			add("consider", "Everything is measured and recorded — close the project",
				"Closing formally frees your attention and marks the benefits as delivered in the portfolio.", "close")
		}
	}

	// Adoption watch after go-live.
	if p.Stage == "benefits" || p.Stage == "implement" {
		for _, b := range s.Benefits {
			if b.Category == "adoption" && len(b.Measurements) > 0 {
				last := b.Latest()
				if b.HasTarget() && !b.Achieved() {
					add("soon", fmt.Sprintf("Adoption is below target (%.0f%s vs target %.0f%s)",
						last.Value, b.Unit, b.TargetValue.Float64, b.Unit),
						"Low adoption quietly cancels the benefits. Talk to the people not using the solution and find out what is in their way.", "benefits")
				}
			}
		}
	}

	// --- Momentum and trends ---

	// A project nobody has touched is a project quietly dying.
	if days, ok := daysSince(s.LastActivityAt); ok && days >= 21 {
		add("soon", fmt.Sprintf("Nothing has been recorded on this project for %d days", days),
			"Stalled projects lose sponsor confidence and team memory. Do one small thing today — or put the project formally on hold.", "overview")
	}

	// Sitting in a stage well past its natural length deserves a question.
	if days, ok := stageAge(s); ok {
		if limit := stageDwellLimit[p.Stage]; limit > 0 && days > limit {
			add("consider", fmt.Sprintf("This project has been in %s for %d days", p.StageName(), days),
				"Long-running stages usually mean a blocker nobody has named. Check the gate checklist: what is actually stopping progress?", "overview")
		}
	}

	// Overdue external dependencies are outside your control but not
	// outside your responsibility to chase.
	for _, ri := range s.Raid {
		if ri.Kind == "dependency" && ri.Status == "open" {
			if d, ok := store.DaysUntil(ri.DueDate); ok && d < 0 {
				add("act", fmt.Sprintf("Dependency %s is %d days past its needed-by date", ri.Ref, -d),
					"External dependencies don't chase themselves. Escalate to the owner today — and consider what plan B looks like.", "raid")
			}
		}
	}

	// Too much work in progress means nothing finishes.
	if n := countTaskStatus(s, "doing"); n > 5 {
		add("consider", fmt.Sprintf("%d tasks are in progress at once", n),
			"Work in progress hides slow progress. Finish things before starting more — flow beats motion.", "board")
	}

	// A benefit trending away from its target is an early warning worth
	// more than any status meeting.
	for _, b := range s.Benefits {
		if len(b.Measurements) < 2 || !b.HasTarget() {
			continue
		}
		prev := b.Measurements[len(b.Measurements)-2].Value
		last := b.Measurements[len(b.Measurements)-1].Value
		worse := last > prev
		if b.Direction == "increase" {
			worse = last < prev
		}
		if worse && !b.Achieved() {
			add("soon", fmt.Sprintf("%s is trending the wrong way (%s → %s %s)",
				b.Ref, trimNum(prev), trimNum(last), b.Unit),
				"Benefits that drift backwards rarely self-correct. Find out what changed before the gain evaporates.", "benefits")
		}
	}

	// Measurements go stale; benefits are proven by fresh numbers.
	if p.Stage == "benefits" {
		for _, b := range s.Benefits {
			last := b.Latest()
			if last == nil || b.Achieved() {
				continue
			}
			if days, ok := daysSince(last.MeasuredAt); ok && days > 30 {
				add("soon", fmt.Sprintf("%s has not been measured for %d days", b.Ref, days),
					"Re-measure on a regular drumbeat until the target is met — otherwise 'benefits realisation' becomes a one-off guess.", "benefits")
			}
		}
	}

	// A high-influence resistant stakeholder can sink the project alone.
	for _, sh := range s.Stakeholders {
		if sh.Influence == "high" && sh.Attitude == "resistant" {
			add("soon", fmt.Sprintf("Win over %s — high influence and currently resistant", sh.Name),
				"Resistance from someone powerful outweighs enthusiasm from everyone else. Listen first: resistance usually contains real information about the design.", "people")
		}
	}

	// Lots of approved change piles pressure on the original plan.
	if p.Stage == "build" && countChanges(s, "approved") >= 3 {
		add("consider", "Several changes have been approved — check the target end date still holds",
			"Approved scope rarely comes free. Re-baseline the plan deliberately rather than silently absorbing the extra work.", "plan")
	}

	// When the gate is clear, say so — momentum is a feature.
	gateNow := CheckGate(s)
	if gateNow.AllMet() && gateNow.NextStage != "" {
		add("consider", "All gate criteria met — ready to move to "+store.StageNames[gateNow.NextStage],
			"Progressing through the gate keeps momentum and unlocks the next set of activities.", "overview")
	}

	sortAdvice(out)
	return out
}

func hasOutOfScope(s *store.Snapshot) bool {
	if filled(s.Project.ScopeOut) {
		return true
	}
	for _, item := range s.ScopeItems {
		if item.Classification == "out" && item.Status != "removed" {
			return true
		}
	}
	return false
}

// stageDwellLimit is the number of days a project can comfortably sit in
// each stage before the coach starts asking questions.
var stageDwellLimit = map[string]int{
	"intake":    14,
	"discovery": 45,
	"define":    30,
	"plan":      21,
	"build":     90,
	"implement": 45,
	"benefits":  90,
}

// stageAge returns days since the project entered its current stage:
// the most recent gate transition, or project creation for Intake.
func stageAge(s *store.Snapshot) (int, bool) {
	if len(s.GateHistory) > 0 {
		return daysSince(s.GateHistory[0].MovedAt)
	}
	return daysSince(s.Project.CreatedAt)
}

// daysSince returns whole days since a stored date or timestamp.
func daysSince(ts string) (int, bool) {
	return ageInDays(ts)
}

func countTaskStatus(s *store.Snapshot, status string) int {
	n := 0
	for _, t := range s.Tasks {
		if t.Status == status {
			n++
		}
	}
	return n
}

func countChanges(s *store.Snapshot, status string) int {
	n := 0
	for _, c := range s.Changes {
		if c.Status == status {
			n++
		}
	}
	return n
}

func trimNum(v float64) string {
	out := fmt.Sprintf("%.1f", v)
	return strings.TrimSuffix(out, ".0")
}

// coachGoLivePrep warns when go-live is near but training work is missing.
func coachGoLivePrep(s *store.Snapshot, add func(severity, message, why, link string)) {
	d, ok := store.DaysUntil(s.Project.GoLive)
	if !ok || d > 21 || d < 0 {
		return
	}
	hasTraining := false
	for _, r := range s.Readiness {
		if r.Category == "training" {
			hasTraining = true
			break
		}
	}
	if !hasTraining {
		add("act", fmt.Sprintf("Go-live is %d days away but no training plan exists", d),
			"People need time to learn a new way of working. Add training items to the readiness checklist and schedule sessions now.", "implementation")
	}
}

func mustRequirementShare(s *store.Snapshot) float64 {
	if len(s.Requirements) == 0 {
		return 0
	}
	musts := 0
	for _, r := range s.Requirements {
		if r.Moscow == "must" {
			musts++
		}
	}
	return float64(musts) / float64(len(s.Requirements))
}

// ageInDays returns whole days since a stored timestamp ("YYYY-MM-DD..."),
func ageInDays(timestamp string) (int, bool) {
	if len(timestamp) < 10 {
		return 0, false
	}
	d, ok := store.DaysUntil(timestamp[:10])
	if !ok {
		return 0, false
	}
	return -d, true
}

func sortAdvice(items []Advice) {
	// Simple stable insertion sort by severity; lists are short.
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && severityRank[items[j].Severity] < severityRank[items[j-1].Severity]; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// NextAction returns the single most important recommendation, used on
// dashboards. ok is false when the coach has nothing to say.
func NextAction(s *store.Snapshot) (Advice, bool) {
	items := Advise(s)
	if len(items) == 0 {
		return Advice{}, false
	}
	return items[0], true
}
