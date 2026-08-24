package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"digipm/internal/db"
	"digipm/internal/store"
)

func testHTTPServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "test.db")
	sqldb, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	st := store.New(sqldb)
	srv, err := NewServer(st, dataDir, filepath.Join(dataDir, "backups"), dbPath, "test")
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	return srv, st
}

func performRequest(handler http.Handler, method, target string, form url.Values) *httptest.ResponseRecorder {
	var body *strings.Reader
	if form == nil {
		body = strings.NewReader("")
	} else {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestCorePagesRender(t *testing.T) {
	srv, _ := testHTTPServer(t)
	handler := srv.Handler()
	for _, path := range []string{"/", "/ideas", "/projects", "/settings"} {
		rr := performRequest(handler, http.MethodGet, path, nil)
		if rr.Code != http.StatusOK {
			t.Errorf("GET %s returned %d: %s", path, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
			t.Errorf("GET %s content type = %q", path, rr.Header().Get("Content-Type"))
		}
	}
}

func TestProjectCreationHTTPWorkflow(t *testing.T) {
	srv, st := testHTTPServer(t)
	handler := srv.Handler()
	rr := performRequest(handler, http.MethodPost, "/projects", url.Values{
		"name":              {"Line-side traceability"},
		"sponsor":           {"Operations Director"},
		"lead":              {"Alex"},
		"department":        {"Operations"},
		"problem_statement": {"Paper records delay investigations"},
		"goal":              {"Trace every assembly digitally"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("create project returned %d: %s", rr.Code, rr.Body.String())
	}
	location := rr.Header().Get("Location")
	if location != "/projects/1/overview" {
		t.Fatalf("create redirect = %q", location)
	}
	projects, err := st.Projects()
	if err != nil || len(projects) != 1 {
		t.Fatalf("stored projects = %d, err=%v", len(projects), err)
	}
	rr = performRequest(handler, http.MethodGet, location, nil)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Line-side traceability") {
		t.Fatalf("project page returned %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSidebarColourSettingWorkflow(t *testing.T) {
	srv, st := testHTTPServer(t)
	handler := srv.Handler()
	rr := performRequest(handler, http.MethodPost, "/settings", url.Values{
		"org_name":      {"HydraForce"},
		"currency":      {"£"},
		"hourly_rate":   {"30"},
		"sidebar_color": {"#0057b8"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("save settings returned %d: %s", rr.Code, rr.Body.String())
	}
	if got := st.Setting("sidebar_color", ""); got != "#0057B8" {
		t.Fatalf("stored sidebar colour = %q", got)
	}
	rr = performRequest(handler, http.MethodGet, "/", nil)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "--sidebar-bg:#0057B8") {
		t.Fatalf("dashboard did not render saved sidebar colour: %s", rr.Body.String())
	}
}

func TestScopeControlHTTPWorkflow(t *testing.T) {
	srv, st := testHTTPServer(t)
	p, err := st.CreateProject("Scope control", "Sponsor", "PM", "Ops", "A sufficiently detailed problem", "Improve it")
	if err != nil {
		t.Fatal(err)
	}
	req, err := st.CreateRequirement(p.ID, "Retain audit trail", "", "must", "Workshop")
	if err != nil {
		t.Fatal(err)
	}
	handler := srv.Handler()
	projectPath := "/projects/" + itoa64(p.ID)
	rr := performRequest(handler, http.MethodPost, projectPath+"/scope", url.Values{
		"classification": {"in"}, "title": {"Digitise approvals"}, "owner": {"Process owner"},
		"rationale": {"Remove paper"}, "acceptance_criteria": {"Approval is searchable"}, "status": {"agreed"},
		"back": {projectPath + "/case"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("create scope returned %d: %s", rr.Code, rr.Body.String())
	}
	items, _ := st.ScopeItems(p.ID)
	if len(items) != 1 {
		t.Fatalf("scope items = %d", len(items))
	}
	rr = performRequest(handler, http.MethodPost, projectPath+"/scope/baselines", url.Values{
		"approved_by": {"Sponsor"}, "approved_at": {store.Today()}, "notes": {"Steering approval"},
		"back": {projectPath + "/case"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("approve baseline returned %d: %s", rr.Code, rr.Body.String())
	}
	rr = performRequest(handler, http.MethodPost, projectPath+"/changes", url.Values{
		"title": {"Add escalation"}, "cost_impact": {"1500"}, "schedule_impact_days": {"4"},
		"scope_item_ids": {itoa64(items[0].ID)}, "requirement_ids": {itoa64(req.ID)},
		"back": {projectPath + "/changes"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("create change returned %d: %s", rr.Code, rr.Body.String())
	}
	changes, err := st.ChangeRequests(p.ID)
	if err != nil || len(changes) != 1 || !changes[0].AffectsScope(items[0].ID) || !changes[0].AffectsRequirement(req.ID) {
		t.Fatalf("change links not stored: %+v err=%v", changes, err)
	}
	rr = performRequest(handler, http.MethodGet, projectPath+"/case", nil)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Scope baseline approval") {
		t.Fatalf("case page did not render scope control: %d %s", rr.Code, rr.Body.String())
	}
	rr = performRequest(handler, http.MethodGet, projectPath+"/changes", nil)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "SCP-001") || !strings.Contains(rr.Body.String(), "REQ-001") {
		t.Fatalf("changes page did not render traceability: %d %s", rr.Code, rr.Body.String())
	}
}
