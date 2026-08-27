// Package pdf renders branded, print-ready project documents.
package pdf

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/coach"
	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
)

// Brand palette (matches the web UI).
var (
	wine  = rgb{92, 30, 48}
	brick = rgb{157, 57, 59}
	clay  = rgb{154, 82, 71}
	rose  = rgb{248, 239, 242}
	ink   = rgb{38, 25, 29}
	grey  = rgb{95, 83, 86}
	line  = rgb{233, 226, 227}
	green = rgb{30, 123, 79}
	amber = rgb{143, 100, 0}
	red   = rgb{180, 50, 44}
)

type rgb struct{ r, g, b int }

// doc wraps fpdf with the small set of layout helpers these documents use.
type doc struct {
	pdf *fpdf.Fpdf
	tr  func(string) string
}

const (
	pageWidth  = 210.0
	marginX    = 18.0
	usable     = pageWidth - 2*marginX
	lineHeight = 5.2
)

func newDoc() *doc {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(marginX, 14, marginX)
	p.SetAutoPageBreak(true, 20)
	d := &doc{pdf: p, tr: p.UnicodeTranslatorFromDescriptor("")}
	p.SetFooterFunc(func() {
		p.SetY(-14)
		p.SetFont("Helvetica", "", 8)
		p.SetTextColor(grey.r, grey.g, grey.b)
		p.CellFormat(0, 5, d.tr(fmt.Sprintf("Page %d", p.PageNo())), "", 0, "C", false, 0, "")
	})
	return d
}

// header draws the wine title band used on page one of every document.
func (d *doc) header(docType, title, subtitle string) {
	p := d.pdf
	p.SetFillColor(wine.r, wine.g, wine.b)
	p.Rect(0, 0, pageWidth, 34, "F")
	p.SetXY(marginX, 8)
	p.SetFont("Helvetica", "", 9)
	p.SetTextColor(216, 172, 185)
	p.CellFormat(0, 5, d.tr(strings.ToUpper(docType)), "", 1, "L", false, 0, "")
	p.SetX(marginX)
	p.SetFont("Helvetica", "B", 17)
	p.SetTextColor(255, 255, 255)
	p.CellFormat(0, 8, d.tr(title), "", 1, "L", false, 0, "")
	p.SetX(marginX)
	p.SetFont("Helvetica", "", 9.5)
	p.SetTextColor(237, 217, 224)
	p.CellFormat(0, 5, d.tr(subtitle), "", 1, "L", false, 0, "")
	p.SetY(42)
}

func (d *doc) section(title string) {
	p := d.pdf
	if p.GetY() > 250 {
		p.AddPage()
	}
	p.Ln(3)
	p.SetFont("Helvetica", "B", 11.5)
	p.SetTextColor(brick.r, brick.g, brick.b)
	p.CellFormat(0, 6, d.tr(title), "", 1, "L", false, 0, "")
	p.SetDrawColor(line.r, line.g, line.b)
	p.SetLineWidth(0.3)
	p.Line(marginX, p.GetY(), pageWidth-marginX, p.GetY())
	p.Ln(2.5)
}

func (d *doc) para(text string) {
	if strings.TrimSpace(text) == "" {
		d.muted("Not recorded.")
		return
	}
	p := d.pdf
	p.SetFont("Helvetica", "", 9.5)
	p.SetTextColor(ink.r, ink.g, ink.b)
	p.MultiCell(usable, lineHeight, d.tr(strings.ReplaceAll(text, "\r\n", "\n")), "", "L", false)
	p.Ln(1)
}

func (d *doc) muted(text string) {
	p := d.pdf
	p.SetFont("Helvetica", "I", 9)
	p.SetTextColor(grey.r, grey.g, grey.b)
	p.MultiCell(usable, lineHeight, d.tr(text), "", "L", false)
	p.Ln(1)
}

// metaRow prints a label/value pair on one line.
func (d *doc) metaRow(pairs ...[2]string) {
	p := d.pdf
	p.SetFont("Helvetica", "", 9.5)
	w := usable / float64(len(pairs))
	for _, pair := range pairs {
		x, y := p.GetX(), p.GetY()
		p.SetTextColor(grey.r, grey.g, grey.b)
		p.CellFormat(w, 5, d.tr(pair[0]), "", 0, "L", false, 0, "")
		p.SetXY(x, y+4.5)
		p.SetFont("Helvetica", "B", 10)
		p.SetTextColor(ink.r, ink.g, ink.b)
		p.CellFormat(w, 5, d.tr(pair[1]), "", 0, "L", false, 0, "")
		p.SetFont("Helvetica", "", 9.5)
		p.SetXY(x+w, y)
	}
	p.Ln(11)
}

// table renders a simple striped table. widths must sum to <= usable.
func (d *doc) table(headers []string, widths []float64, rows [][]string) {
	p := d.pdf
	p.SetFont("Helvetica", "B", 8.5)
	p.SetTextColor(grey.r, grey.g, grey.b)
	p.SetFillColor(rose.r, rose.g, rose.b)
	for i, h := range headers {
		p.CellFormat(widths[i], 6.5, d.tr(strings.ToUpper(h)), "", 0, "L", true, 0, "")
	}
	p.Ln(-1)
	p.SetFont("Helvetica", "", 9)
	p.SetTextColor(ink.r, ink.g, ink.b)
	p.SetDrawColor(line.r, line.g, line.b)
	for _, row := range rows {
		if p.GetY() > 262 {
			p.AddPage()
		}
		// Measure the tallest cell to keep the row aligned.
		maxLines := 1
		for i, cell := range row {
			n := measureLines(p, cell, widths[i]-2)
			if n > maxLines {
				maxLines = n
			}
		}
		rowH := float64(maxLines)*4.6 + 2.2
		x, y := p.GetX(), p.GetY()
		for i, cell := range row {
			p.SetXY(x, y+1.1)
			p.MultiCell(widths[i]-2, 4.6, d.tr(cell), "", "L", false)
			x += widths[i]
			p.SetXY(x, y)
		}
		p.SetXY(marginX, y+rowH)
		p.Line(marginX, y+rowH, marginX+sum(widths), y+rowH)
	}
	p.Ln(2)
}

// measureLines counts how many lines a cell will wrap to. SplitText only
// supports Latin-1 code points, so wider runes are swapped for a same-width
// placeholder before measuring; rendering still uses the translator.
func measureLines(p *fpdf.Fpdf, s string, width float64) int {
	safe := make([]rune, 0, len(s))
	for _, r := range s {
		if r > 0xFF {
			r = '?'
		}
		safe = append(safe, r)
	}
	n := len(p.SplitText(string(safe), width))
	if n < 1 {
		n = 1
	}
	return n
}

func sum(vs []float64) float64 {
	t := 0.0
	for _, v := range vs {
		t += v
	}
	return t
}

func dateOr(iso, fallback string) string {
	if iso == "" {
		return fallback
	}
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("2 Jan 2006")
}

func numf(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.1f", v)
}

func moneyf(currency string, v float64) string {
	s := fmt.Sprintf("%.0f", v)
	// Thousands separators.
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return currency + s
}

func benefitRange(b store.Benefit) (baseline, target, actual string) {
	baseline, target, actual = "—", "—", "—"
	if b.HasBaseline() {
		baseline = numf(b.BaselineValue.Float64) + " " + b.Unit
	}
	if b.HasTarget() {
		target = numf(b.TargetValue.Float64) + " " + b.Unit
	}
	if last := b.Latest(); last != nil {
		actual = numf(last.Value) + " (" + dateOr(last.MeasuredAt, "") + ")"
	}
	return
}

// BusinessCase renders the business-case document for a project.
func BusinessCase(w io.Writer, snap *store.Snapshot, currency, orgName string) error {
	d := newDoc()
	p := d.pdf
	sub := snap.Project.Code
	if orgName != "" {
		sub += "  ·  " + orgName
	}
	sub += "  ·  " + time.Now().Format("2 Jan 2006")
	p.AddPage()
	d.header("Business case", snap.Project.Name, sub)

	d.metaRow(
		[2]string{"Sponsor", orDash(snap.Project.Sponsor)},
		[2]string{"Project lead", orDash(snap.Project.Lead)},
		[2]string{"Department", orDash(snap.Project.Department)},
		[2]string{"Target end", dateOr(snap.Project.TargetEnd, "—")},
	)

	d.section("The problem")
	d.para(snap.Project.ProblemStatement)

	if len(snap.PainPoints) > 0 {
		var rows [][]string
		for _, pp := range snap.PainPoints {
			rows = append(rows, []string{pp.Description, pp.ProcessArea, capitalize(pp.Impact), pp.Frequency})
		}
		d.section("Evidence: pain points from discovery")
		d.table([]string{"Pain point", "Process area", "Impact", "Frequency"},
			[]float64{92, 34, 20, 28}, rows)
	}

	d.section("Desired outcome")
	d.para(snap.Project.Goal)

	d.section("Proposal and justification")
	d.para(snap.Project.BusinessCase)

	d.section("Expected benefits")
	if len(snap.Benefits) == 0 {
		d.muted("No measurable benefits defined yet.")
	} else {
		var rows [][]string
		var annual float64
		for _, b := range snap.Benefits {
			baseline, target, _ := benefitRange(b)
			val := "—"
			if b.AnnualValue > 0 {
				val = moneyf(currency, b.AnnualValue) + "/yr"
				annual += b.AnnualValue
			}
			rows = append(rows, []string{b.Ref + "  " + b.Name, baseline, target, val})
		}
		d.table([]string{"Benefit", "Baseline (today)", "Target", "Value if achieved"},
			[]float64{80, 32, 30, 32}, rows)
		if annual > 0 {
			p.SetFont("Helvetica", "B", 10)
			p.SetTextColor(wine.r, wine.g, wine.b)
			p.CellFormat(0, 6, d.tr("Total expected annual value: "+moneyf(currency, annual)), "", 1, "L", false, 0, "")
		}
	}

	finance := store.SummariseFinancials(snap.Financials, snap.Benefits)
	d.section("Investment, ROI and payback")
	roi, payback := "—", "—"
	if finance.HasROI {
		roi = fmt.Sprintf("%.0f%%", finance.ROI)
	}
	if finance.HasPayback {
		payback = fmt.Sprintf("%.1f months", finance.PaybackMonths)
	}
	d.table([]string{"Estimate", "Approved budget", "Actual cost", "First-year ROI", "Payback"},
		[]float64{34, 36, 34, 34, 36}, [][]string{{
			moneyf(currency, snap.Financials.EstimatedCost), moneyf(currency, snap.Financials.ApprovedBudget),
			moneyf(currency, snap.Financials.ActualCost), roi, payback,
		}})
	if strings.TrimSpace(snap.Financials.Notes) != "" {
		d.para(snap.Financials.Notes)
	}

	d.section("Scope")
	d.scopeColumns(snap.Project.ScopeIn, snap.Project.ScopeOut)
	if len(snap.ScopeItems) > 0 {
		var rows [][]string
		for _, item := range snap.ScopeItems {
			if item.Status == "removed" {
				continue
			}
			boundary := "In"
			if item.Classification == "out" {
				boundary = "Out"
			}
			acceptance := item.AcceptanceCriteria
			if strings.TrimSpace(acceptance) == "" {
				acceptance = "—"
			}
			rows = append(rows, []string{item.Ref, boundary, item.Title, orDash(item.Owner), acceptance})
		}
		if len(rows) > 0 {
			d.table([]string{"Ref", "Boundary", "Scope item", "Owner", "Acceptance criteria"},
				[]float64{20, 18, 54, 30, 52}, rows)
		}
	}

	if reqs := keyRequirements(snap); len(reqs) > 0 {
		d.section("Key requirements (must have)")
		for _, r := range reqs {
			d.bullet(r)
		}
	}

	d.section("Approval")
	if len(snap.ScopeBaselines) > 0 {
		latest := snap.ScopeBaselines[0]
		d.metaRow(
			[2]string{"Scope baseline", fmt.Sprintf("Version %d", latest.Version)},
			[2]string{"Approved by", latest.ApprovedBy},
			[2]string{"Approved", dateOr(latest.ApprovedAt, "—")},
		)
		if latest.Notes != "" {
			d.para(latest.Notes)
		}
	} else {
		d.muted("Scope baseline not yet approved. Sponsor signature required to proceed to planning and delivery.")
		p.Ln(6)
		d.signatureLine("Sponsor", orDash(snap.Project.Sponsor))
	}

	return p.Output(w)
}

func (d *doc) bullet(text string) {
	p := d.pdf
	p.SetFont("Helvetica", "", 9.5)
	p.SetTextColor(ink.r, ink.g, ink.b)
	x := p.GetX()
	p.CellFormat(5, lineHeight, d.tr("•"), "", 0, "L", false, 0, "")
	p.MultiCell(usable-5, lineHeight, d.tr(text), "", "L", false)
	p.SetX(x)
}

func (d *doc) scopeColumns(in, out string) {
	p := d.pdf
	colW := usable/2 - 4
	p.SetFont("Helvetica", "B", 9.5)
	p.SetTextColor(green.r, green.g, green.b)
	p.CellFormat(colW, 5.5, d.tr("In scope"), "", 0, "L", false, 0, "")
	p.SetX(marginX + colW + 8)
	p.SetTextColor(red.r, red.g, red.b)
	p.CellFormat(colW, 5.5, d.tr("Out of scope"), "", 1, "L", false, 0, "")

	p.SetFont("Helvetica", "", 9)
	p.SetTextColor(ink.r, ink.g, ink.b)
	topY := p.GetY()
	p.SetXY(marginX, topY)
	p.MultiCell(colW, 4.8, d.tr(orDash(strings.TrimSpace(in))), "", "L", false)
	leftEnd := p.GetY()
	p.SetXY(marginX+colW+8, topY)
	p.MultiCell(colW, 4.8, d.tr(orDash(strings.TrimSpace(out))), "", "L", false)
	rightEnd := p.GetY()
	if leftEnd > rightEnd {
		p.SetY(leftEnd)
	} else {
		p.SetY(rightEnd)
	}
	p.Ln(1)
}

func (d *doc) signatureLine(role, name string) {
	p := d.pdf
	p.SetDrawColor(grey.r, grey.g, grey.b)
	p.SetLineWidth(0.2)
	y := p.GetY() + 10
	p.Line(marginX, y, marginX+70, y)
	p.Line(marginX+95, y, marginX+135, y)
	p.SetXY(marginX, y+1)
	p.SetFont("Helvetica", "", 8.5)
	p.SetTextColor(grey.r, grey.g, grey.b)
	p.CellFormat(70, 5, d.tr(role+": "+name), "", 0, "L", false, 0, "")
	p.SetX(marginX + 95)
	p.CellFormat(40, 5, d.tr("Date"), "", 1, "L", false, 0, "")
}

func keyRequirements(snap *store.Snapshot) []string {
	var out []string
	for _, r := range snap.Requirements {
		if r.Moscow == "must" && r.Status != "dropped" {
			out = append(out, r.Ref+"  "+r.Title)
		}
	}
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// StatusReport renders a one-to-two-page status report for a project.
func StatusReport(w io.Writer, snap *store.Snapshot, health coach.Health, gate coach.GateCheck, currency, orgName string) error {
	d := newDoc()
	p := d.pdf
	proj := snap.Project
	sub := proj.Code + "  ·  " + proj.StageName()
	if orgName != "" {
		sub += "  ·  " + orgName
	}
	sub += "  ·  " + time.Now().Format("2 Jan 2006")
	p.AddPage()
	d.header("Project status report", proj.Name, sub)

	// Health badge.
	label, col := healthBadge(health)
	p.SetFillColor(col.r, col.g, col.b)
	p.SetTextColor(255, 255, 255)
	p.SetFont("Helvetica", "B", 9.5)
	w1 := p.GetStringWidth(d.tr(label)) + 10
	p.CellFormat(w1, 7, d.tr(label), "", 0, "C", true, 0, "")
	p.SetTextColor(grey.r, grey.g, grey.b)
	p.SetFont("Helvetica", "", 9)
	gateMsg := fmt.Sprintf("   Gate: %d of %d criteria met", gate.MetCount(), len(gate.Criteria))
	if gate.NextStage != "" {
		gateMsg += " toward " + store.StageNames[gate.NextStage]
	}
	p.CellFormat(0, 7, d.tr(gateMsg), "", 1, "L", false, 0, "")
	p.Ln(3)

	d.metaRow(
		[2]string{"Sponsor", orDash(proj.Sponsor)},
		[2]string{"Lead", orDash(proj.Lead)},
		[2]string{"Started", dateOr(proj.StartDate, "—")},
		[2]string{"Target end", dateOr(proj.TargetEnd, "—")},
	)

	if len(health.Reasons) > 0 {
		d.section("Why this rating")
		for _, r := range health.Reasons {
			d.bullet(r)
		}
	}

	finance := store.SummariseFinancials(snap.Financials, snap.Benefits)
	d.section("Financial case")
	roi, payback := "—", "—"
	if finance.HasROI {
		roi = fmt.Sprintf("%.0f%%", finance.ROI)
	}
	if finance.HasPayback {
		payback = fmt.Sprintf("%.1f months", finance.PaybackMonths)
	}
	d.table([]string{"Investment", "Expected annual value", "Realised annual value", "ROI", "Payback"},
		[]float64{35, 43, 43, 22, 31}, [][]string{{
			moneyf(currency, finance.Investment), moneyf(currency, finance.ExpectedAnnualValue),
			moneyf(currency, finance.RealisedAnnualValue), roi, payback,
		}})

	d.section("Milestones")
	if len(snap.Milestones) == 0 {
		d.muted("No milestones defined.")
	} else {
		var rows [][]string
		for _, m := range snap.Milestones {
			status := "Planned"
			if m.Done() {
				status = "Done " + dateOr(m.CompletedAt, "")
			} else if m.Overdue() {
				status = "OVERDUE"
			}
			rows = append(rows, []string{m.Name, dateOr(m.DueDate, "—"), status})
		}
		d.table([]string{"Milestone", "Due", "Status"}, []float64{104, 34, 36}, rows)
	}

	d.section("Top open risks and issues")
	openItems := topOpenRaid(snap, 6)
	if len(openItems) == 0 {
		d.muted("No open risks or issues.")
	} else {
		var rows [][]string
		for _, ri := range openItems {
			mit := ri.Mitigation
			if strings.TrimSpace(mit) == "" {
				mit = "— no mitigation recorded —"
			}
			rows = append(rows, []string{ri.Ref, ri.Title, fmt.Sprintf("%d", ri.Score()), mit})
		}
		d.table([]string{"Ref", "Item", "Score", "Mitigation / plan"},
			[]float64{20, 66, 14, 74}, rows)
	}

	d.section("Benefits")
	if len(snap.Benefits) == 0 {
		d.muted("No benefits defined.")
	} else {
		var rows [][]string
		for _, b := range snap.Benefits {
			baseline, target, actual := benefitRange(b)
			status := "Not measured"
			if b.Achieved() {
				status = "Achieved"
			} else if b.Latest() != nil {
				status = "In progress"
			}
			rows = append(rows, []string{b.Ref + "  " + b.Name, baseline, target, actual, status})
		}
		d.table([]string{"Benefit", "Baseline", "Target", "Latest actual", "Status"},
			[]float64{62, 28, 26, 34, 24}, rows)
	}

	// Delivery snapshot.
	done := 0
	for _, t := range snap.Tasks {
		if t.Status == "done" {
			done++
		}
	}
	d.section("Delivery snapshot")
	d.bullet(fmt.Sprintf("Tasks: %d done of %d", done, len(snap.Tasks)))
	if musts, passing := mustTestStats(snap); musts > 0 {
		d.bullet(fmt.Sprintf("Must-have requirements passing tests: %d of %d", passing, musts))
	}
	if len(snap.Readiness) > 0 {
		rd := 0
		for _, ri := range snap.Readiness {
			if ri.Done {
				rd++
			}
		}
		d.bullet(fmt.Sprintf("Go-live readiness: %d of %d items complete", rd, len(snap.Readiness)))
	}

	return p.Output(w)
}

func healthBadge(h coach.Health) (string, rgb) {
	switch h.Status {
	case "green":
		return "ON TRACK", green
	case "amber":
		return "AT RISK", amber
	case "red":
		return "NEEDS ATTENTION", red
	default:
		return "CLOSED", grey
	}
}

func topOpenRaid(snap *store.Snapshot, n int) []store.RaidItem {
	var out []store.RaidItem
	for _, ri := range snap.Raid {
		if ri.Status == "open" && (ri.Kind == "risk" || ri.Kind == "issue") {
			out = append(out, ri)
			if len(out) == n {
				break
			}
		}
	}
	return out
}

func mustTestStats(snap *store.Snapshot) (musts, passing int) {
	for _, r := range snap.Requirements {
		if r.Moscow != "must" || r.Status == "dropped" {
			continue
		}
		musts++
		if r.TestSummary() == "passing" {
			passing++
		}
	}
	return
}
