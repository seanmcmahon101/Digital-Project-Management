package pdf

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/coach"
	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
)

func nf(v float64) sql.NullFloat64 { return sql.NullFloat64{Float64: v, Valid: true} }

func sampleSnapshot() *store.Snapshot {
	return &store.Snapshot{
		Project: store.Project{
			ID: 1, Code: "DPM-001", Name: "OEE Roll Out", Stage: "define", Status: "active",
			Sponsor: "Philipp & Nick", Lead: "Sean M",
			ProblemStatement: "Machine performance is invisible; downtime causes are anecdotal.",
			Goal:             "Live OEE per cell with downtime reasons.",
			BusinessCase:     "OEE visibility unlocks an estimated £40k/yr of recoverable capacity.",
			ScopeIn:          "Cells 1-5\nDowntime reasons",
			ScopeOut:         "Assembly\nEnergy monitoring",
			StartDate:        "2026-06-01", TargetEnd: "2026-12-31",
		},
		PainPoints: []store.PainPoint{
			{Description: "Downtime reasons are guessed at month end", ProcessArea: "Production", Impact: "high", Frequency: "Daily"},
		},
		Benefits: []store.Benefit{{
			Ref: "BEN-001", Name: "OEE", Category: "custom", Unit: "%", Direction: "increase",
			BaselineValue: nf(61), TargetValue: nf(72), AnnualValue: 40000,
			Measurements: []store.BenefitMeasurement{{Value: 65, MeasuredAt: "2026-08-01"}},
		}},
		Requirements: []store.Requirement{
			{Ref: "REQ-001", Title: "Capture downtime reason at the machine", Moscow: "must"},
		},
		Milestones: []store.Milestone{
			{Name: "Pilot cell live", DueDate: "2026-09-15"},
		},
		Raid: []store.RaidItem{
			{Ref: "RISK-001", Kind: "risk", Title: "Operators don't log reasons", Status: "open", Probability: 3, Impact: 4, Mitigation: "One-tap reason buttons"},
		},
	}
}

// pdfStart checks the output is a plausible, non-trivial PDF.
func pdfStart(t *testing.T, buf *bytes.Buffer, minSize int) {
	t.Helper()
	if !strings.HasPrefix(buf.String(), "%PDF-") {
		t.Fatalf("output does not start with %%PDF- header")
	}
	if buf.Len() < minSize {
		t.Fatalf("PDF suspiciously small: %d bytes", buf.Len())
	}
	if !strings.Contains(buf.String(), "%%EOF") {
		t.Fatal("PDF missing EOF marker")
	}
}

func TestBusinessCasePDF(t *testing.T) {
	var buf bytes.Buffer
	if err := BusinessCase(&buf, sampleSnapshot(), "£", "HydraForce"); err != nil {
		t.Fatalf("BusinessCase: %v", err)
	}
	pdfStart(t, &buf, 2000)
}

func TestBusinessCasePDFOnEmptyProject(t *testing.T) {
	snap := &store.Snapshot{Project: store.Project{ID: 1, Code: "DPM-002", Name: "Bare", Stage: "intake", Status: "active"}}
	var buf bytes.Buffer
	if err := BusinessCase(&buf, snap, "£", ""); err != nil {
		t.Fatalf("BusinessCase on empty project: %v", err)
	}
	pdfStart(t, &buf, 1000)
}

func TestStatusReportPDF(t *testing.T) {
	snap := sampleSnapshot()
	health := coach.Assess(snap)
	gate := coach.CheckGate(snap)
	var buf bytes.Buffer
	if err := StatusReport(&buf, snap, health, gate, "£", "HydraForce"); err != nil {
		t.Fatalf("StatusReport: %v", err)
	}
	pdfStart(t, &buf, 2000)
}
