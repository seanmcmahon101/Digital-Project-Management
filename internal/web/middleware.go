package web

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"
)

const contentSecurityPolicy = "default-src 'self'; base-uri 'none'; object-src 'none'; " +
	"frame-ancestors 'none'; form-action 'self'; img-src 'self' data:; font-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'"

// protectHTTP applies the security boundary for this loopback-only app and
// keeps handler failures from tearing down a client connection.
func (s *Server) protectHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w.Header())
		started := time.Now()
		rw := &responseStatusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic handling %s %s: %v\n%s", r.Method, r.URL.Path, recovered, debug.Stack())
				if !rw.wroteHeader {
					http.Error(rw, "Internal error — see log for details.", http.StatusInternalServerError)
				}
			}
			if !strings.HasPrefix(r.URL.Path, "/static/") && r.URL.Path != "/healthz" {
				log.Printf("http: %s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(started).Round(time.Millisecond))
			}
		}()

		if !loopbackHost(r.Host) {
			http.Error(rw, "Invalid host", http.StatusMisdirectedRequest)
			return
		}
		if unsafeMethod(r.Method) && !sameOriginRequest(r) {
			http.Error(rw, "Cross-origin request blocked", http.StatusForbidden)
			return
		}
		next.ServeHTTP(rw, r)
	})
}

func setSecurityHeaders(header http.Header) {
	header.Set("Content-Security-Policy", contentSecurityPolicy)
	header.Set("Cross-Origin-Opener-Policy", "same-origin")
	header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
}

func loopbackHost(value string) bool {
	host := value
	if parsed, _, err := net.SplitHostPort(value); err == nil {
		host = parsed
	} else if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
	} else if strings.Contains(value, ":") {
		return false
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func unsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func sameOriginRequest(r *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))) {
	case "cross-site", "same-site":
		return false
	case "same-origin", "none", "":
	default:
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// Non-browser clients do not send Fetch Metadata. Keeping this case
		// scriptable is useful for local automation; browser POSTs send Origin.
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Host == "" || u.Path != "" {
		return false
	}
	return strings.EqualFold(canonicalAuthority(u.Host, u.Scheme), canonicalAuthority(r.Host, "http"))
}

func canonicalAuthority(authority, scheme string) string {
	host, port, err := net.SplitHostPort(authority)
	if err != nil {
		host = authority
		port = ""
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
		}
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if port == "" {
		if scheme == "http" {
			port = "80"
		} else if scheme == "https" {
			port = "443"
		}
	}
	return fmt.Sprintf("%s:%s", host, port)
}

type responseStatusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *responseStatusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseStatusWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
