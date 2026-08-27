package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
)

//go:embed all:templates all:static
var assets embed.FS

// view is the payload every template receives.
type view struct {
	Title             string
	Version           string
	Active            string // sidebar section key
	Flash             string
	FlashErr          string
	Currency          string
	SidebarColor      string
	SidebarColorDeep  string
	SidebarForeground string
	Data              map[string]any
}

func (s *Server) funcs() template.FuncMap {
	return template.FuncMap{
		"stageName":           func(id string) string { return store.StageNames[id] },
		"stages":              func() []string { return store.Stages },
		"taskStatuses":        func() []string { return store.TaskStatuses },
		"stageIdx":            store.StageIndex,
		"taskStatusName":      func(id string) string { return store.TaskStatusNames[id] },
		"testStatusName":      func(id string) string { return store.TestStatusNames[id] },
		"moscowName":          func(id string) string { return store.MoscowNames[id] },
		"raidKindName":        func(id string) string { return store.RaidKindNames[id] },
		"benefitCategoryName": func(id string) string { return store.BenefitCategoryNames[id] },
		"date": func(iso string) string {
			if iso == "" {
				return "—"
			}
			t, err := time.Parse("2006-01-02", iso)
			if err != nil {
				return iso
			}
			return t.Format("2 Jan 2006")
		},
		"datetime": func(ts string) string {
			t, err := time.Parse("2006-01-02 15:04:05", ts)
			if err != nil {
				return ts
			}
			return t.Local().Format("2 Jan 2006 15:04")
		},
		"daysUntil": func(iso string) int {
			d, _ := store.DaysUntil(iso)
			return d
		},
		"num": func(v float64) string {
			if v == math.Trunc(v) {
				return commas(fmt.Sprintf("%.0f", v))
			}
			return commas(fmt.Sprintf("%.1f", v))
		},
		"money": func(v float64) string {
			return s.currency() + commas(fmt.Sprintf("%.0f", v))
		},
		"pct": func(v float64) string { return fmt.Sprintf("%.0f%%", v) },
		"pctOf": func(n, max int) float64 {
			if max <= 0 {
				return 0
			}
			return float64(n) / float64(max) * 100
		},
		"clamp100": func(v float64) float64 {
			if v > 100 {
				return 100
			}
			if v < 0 {
				return 0
			}
			return v
		},
		"dict": func(pairs ...any) (map[string]any, error) {
			if len(pairs)%2 != 0 {
				return nil, fmt.Errorf("dict requires key/value pairs")
			}
			m := map[string]any{}
			for i := 0; i < len(pairs); i += 2 {
				key, ok := pairs[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				m[key] = pairs[i+1]
			}
			return m, nil
		},
		"truncate": func(n int, s string) string {
			if len(s) <= n {
				return s
			}
			cut := s[:n]
			if i := strings.LastIndex(cut, " "); i > n/2 {
				cut = cut[:i]
			}
			return cut + "…"
		},
		"add":               func(a, b int) int { return a + b },
		"sub":               func(a, b int) int { return a - b },
		"lower":             strings.ToLower,
		"today":             store.Today,
		"kb":                func(size int64) float64 { return float64(size) / 1024 },
		"benefitCategories": func() []string { return store.BenefitCategories },
		"progressOf": func(b store.Benefit) struct {
			Pct float64
			OK  bool
		} {
			pct, ok := b.Progress()
			return struct {
				Pct float64
				OK  bool
			}{pct, ok}
		},
	}
}

// commas inserts thousands separators into a plain integer/decimal string.
func commas(s string) string {
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	intPart, frac := s, ""
	if i := strings.Index(s, "."); i >= 0 {
		intPart, frac = s[:i], s[i:]
	}
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	out := b.String() + frac
	if neg {
		out = "-" + out
	}
	return out
}

func (s *Server) currency() string { return s.St.Setting("currency", "£") }

// parseTemplates builds one template set per page, all sharing the layout
// and partials.
func (s *Server) parseTemplates() error {
	pages, err := fs.Glob(assets, "templates/pages/*.html")
	if err != nil {
		return err
	}
	s.tmpl = map[string]*template.Template{}
	for _, page := range pages {
		name := strings.TrimSuffix(strings.TrimPrefix(page, "templates/pages/"), ".html")
		t, err := template.New("layout.html").Funcs(s.funcs()).
			ParseFS(assets, "templates/layout.html", "templates/partials/*.html", page)
		if err != nil {
			return fmt.Errorf("parse %s: %w", page, err)
		}
		s.tmpl[name] = t
	}
	return nil
}

// render writes a full page. Flash cookies are consumed here.
func (s *Server) render(w http.ResponseWriter, r *http.Request, page string, v view) {
	t, ok := s.tmpl[page]
	if !ok {
		s.serverError(w, fmt.Errorf("unknown template %q", page))
		return
	}
	if v.Data == nil {
		v.Data = map[string]any{}
	}
	v.Currency = s.currency()
	v.Version = s.Version
	v.SidebarColor = normaliseHexColor(s.St.Setting("sidebar_color", "#5C1E30"))
	v.SidebarColorDeep = shadeHexColor(v.SidebarColor, 0.78)
	v.SidebarForeground = readableForeground(v.SidebarColor)
	if c, err := r.Cookie("flash"); err == nil && c.Value != "" {
		if msg, err := url.QueryUnescape(c.Value); err == nil {
			v.Flash = msg
		}
		http.SetCookie(w, &http.Cookie{Name: "flash", Value: "", Path: "/", MaxAge: -1})
	}
	if c, err := r.Cookie("flash_err"); err == nil && c.Value != "" {
		if msg, err := url.QueryUnescape(c.Value); err == nil {
			v.FlashErr = msg
		}
		http.SetCookie(w, &http.Cookie{Name: "flash_err", Value: "", Path: "/", MaxAge: -1})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout.html", v); err != nil {
		log.Printf("render %s: %v", page, err)
	}
}

// normaliseHexColor accepts only a six-digit CSS hex colour. Settings are
// persisted as text, so rendering also validates the value defensively.
func normaliseHexColor(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return "#5C1E30"
	}
	if _, err := strconv.ParseUint(value[1:], 16, 24); err != nil {
		return "#5C1E30"
	}
	return strings.ToUpper(value)
}

func shadeHexColor(value string, factor float64) string {
	value = normaliseHexColor(value)
	n, _ := strconv.ParseUint(value[1:], 16, 24)
	r := int(float64((n>>16)&0xff) * factor)
	g := int(float64((n>>8)&0xff) * factor)
	b := int(float64(n&0xff) * factor)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func readableForeground(value string) string {
	value = normaliseHexColor(value)
	n, _ := strconv.ParseUint(value[1:], 16, 24)
	r := relativeChannel(float64((n>>16)&0xff) / 255)
	g := relativeChannel(float64((n>>8)&0xff) / 255)
	b := relativeChannel(float64(n&0xff) / 255)
	background := 0.2126*r + 0.7152*g + 0.0722*b
	whiteContrast := 1.05 / (background + 0.05)
	inkLuminance := 0.2126*relativeChannel(33.0/255) +
		0.7152*relativeChannel(29.0/255) + 0.0722*relativeChannel(31.0/255)
	inkContrast := (background + 0.05) / (inkLuminance + 0.05)
	if inkContrast >= whiteContrast {
		return "#211D1F"
	}
	return "#FFFFFF"
}

func relativeChannel(channel float64) float64 {
	if channel <= 0.04045 {
		return channel / 12.92
	}
	return math.Pow((channel+0.055)/1.055, 2.4)
}

// redirect sends the user on with an optional success flash.
func (s *Server) redirect(w http.ResponseWriter, r *http.Request, to, flash string) {
	if flash != "" {
		http.SetCookie(w, &http.Cookie{Name: "flash", Value: url.QueryEscape(flash), Path: "/", MaxAge: 30})
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// fail flashes an error and redirects back. Validation problems are shown
// verbatim; unexpected errors are logged and summarised.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, back string, err error) {
	msg := "Something went wrong — the change was not saved."
	var ve *store.ValidationError
	if isValidation(err, &ve) {
		msg = ve.Error()
	} else {
		log.Printf("error handling %s %s: %v", r.Method, r.URL.Path, err)
	}
	http.SetCookie(w, &http.Cookie{Name: "flash_err", Value: url.QueryEscape(msg), Path: "/", MaxAge: 30})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func isValidation(err error, target **store.ValidationError) bool {
	for err != nil {
		if ve, ok := err.(*store.ValidationError); ok {
			*target = ve
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func (s *Server) serverError(w http.ResponseWriter, err error) {
	log.Printf("server error: %v", err)
	http.Error(w, "Internal error — see log for details.", http.StatusInternalServerError)
}

func (s *Server) notFound(w http.ResponseWriter) {
	http.Error(w, "Not found", http.StatusNotFound)
}
