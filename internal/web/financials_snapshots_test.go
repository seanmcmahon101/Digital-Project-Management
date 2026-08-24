package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestFinancialsAndStatusSnapshotHTTPWorkflow(t *testing.T) {
	srv, st := testHTTPServer(t)
	p, err := st.CreateProject("Investment workflow", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	handler := srv.Handler()

	rr := performRequest(handler, http.MethodPost, "/projects/1/financials", url.Values{
		"estimated_cost":  {"4500"},
		"approved_budget": {"5000"},
		"actual_cost":     {"1250.50"},
		"financial_notes": {"Pilot licences and integration"},
	})
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/projects/1/case" {
		t.Fatalf("save financials returned %d to %q: %s", rr.Code, rr.Header().Get("Location"), rr.Body.String())
	}
	fin, err := st.Financials(p.ID)
	if err != nil || fin.EstimatedCost != 4500 || fin.ApprovedBudget != 5000 || fin.ActualCost != 1250.50 {
		t.Fatalf("financials not stored: %+v err=%v", fin, err)
	}

	rr = performRequest(handler, http.MethodPost, "/projects/1/snapshots", url.Values{
		"note": {"August steering baseline"},
	})
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/projects/1/overview" {
		t.Fatalf("capture snapshot returned %d to %q: %s", rr.Code, rr.Header().Get("Location"), rr.Body.String())
	}
	history, err := st.StatusHistory(p.ID, 10)
	if err != nil || len(history) != 1 || history[0].Note != "August steering baseline" {
		t.Fatalf("snapshot not captured: %+v err=%v", history, err)
	}
	if history[0].EstimatedCost != 4500 || history[0].ApprovedBudget != 5000 || history[0].ActualCost != 1250.50 {
		t.Fatalf("snapshot did not freeze financial state: %+v", history[0])
	}

	rr = performRequest(handler, http.MethodGet, "/projects/1/overview", nil)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "August steering baseline") {
		t.Fatalf("overview did not render snapshot history: %d %s", rr.Code, rr.Body.String())
	}
}

func TestFinancialsHTTPRejectsInvalidNumber(t *testing.T) {
	srv, st := testHTTPServer(t)
	if _, err := st.CreateProject("Invalid finance", "", "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	rr := performRequest(srv.Handler(), http.MethodPost, "/projects/1/financials", url.Values{
		"estimated_cost": {"not-a-number"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("invalid financial input returned %d", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("Set-Cookie"), "flash_err=") {
		t.Fatal("expected validation error flash cookie")
	}
}
