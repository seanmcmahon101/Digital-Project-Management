package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestLoopbackHostProtectionAndHealthIdentity(t *testing.T) {
	srv, _ := testHTTPServer(t)
	handler := srv.Handler()

	for _, host := range []string{"localhost:8383", "127.0.0.1:8383", "[::1]:8383"} {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Host = host
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("host %q returned %d", host, rr.Code)
		}
		if got := rr.Header().Get(InstanceHeader); got != "DigitalProjectManagement" {
			t.Errorf("host %q identity header = %q", host, got)
		}
		var health map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &health); err != nil {
			t.Errorf("host %q health JSON: %v", host, err)
		} else if health["application"] != "Digital Project Management" || health["status"] != "ok" || health["version"] != "test" {
			t.Errorf("host %q health payload = %#v", host, health)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Host = "attacker.example"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMisdirectedRequest {
		t.Fatalf("non-loopback Host returned %d, want %d", rr.Code, http.StatusMisdirectedRequest)
	}
}

func TestSecurityHeaders(t *testing.T) {
	srv, _ := testHTTPServer(t)
	rr := performRequest(srv.Handler(), http.MethodGet, "/", nil)
	for name, want := range map[string]string{
		"Content-Security-Policy":    "frame-ancestors 'none'",
		"Referrer-Policy":            "no-referrer",
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
		"Cross-Origin-Opener-Policy": "same-origin",
	} {
		if got := rr.Header().Get(name); !strings.Contains(got, want) {
			t.Errorf("%s = %q, want it to contain %q", name, got, want)
		}
	}
}

func TestUnsafeRequestsRequireBrowserSameOrigin(t *testing.T) {
	tests := []struct {
		name      string
		fetchSite string
		origin    string
		want      int
	}{
		{name: "same origin metadata", fetchSite: "same-origin", origin: "http://127.0.0.1:8383", want: http.StatusSeeOther},
		{name: "cross site metadata", fetchSite: "cross-site", origin: "https://attacker.example", want: http.StatusForbidden},
		{name: "same site is not same origin", fetchSite: "same-site", origin: "http://localhost:9999", want: http.StatusForbidden},
		{name: "mismatched origin", fetchSite: "same-origin", origin: "http://localhost:8383", want: http.StatusForbidden},
		{name: "opaque origin", origin: "null", want: http.StatusForbidden},
		{name: "local automation without browser metadata", want: http.StatusSeeOther},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, st := testHTTPServer(t)
			body := strings.NewReader(url.Values{"name": {"Protected project"}}.Encode())
			req := httptest.NewRequest(http.MethodPost, "/projects", body)
			req.Host = "127.0.0.1:8383"
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if tc.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tc.fetchSite)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rr := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rr.Code, tc.want, rr.Body.String())
			}
			projects, err := st.Projects()
			if err != nil {
				t.Fatal(err)
			}
			wantCreated := tc.want == http.StatusSeeOther
			if (len(projects) == 1) != wantCreated {
				t.Fatalf("projects = %d, want created=%v", len(projects), wantCreated)
			}
		})
	}
}

func TestHTTPRecoveryReturnsInternalServerError(t *testing.T) {
	srv, _ := testHTTPServer(t)
	handler := srv.protectHTTP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}))
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Host = "localhost:8383"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}
