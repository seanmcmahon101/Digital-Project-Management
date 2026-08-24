package web

import (
	"html/template"
	"io/fs"
	"net/http"
	"strconv"

	"digipm/internal/store"
)

// Server wires the store to HTTP handlers and templates.
type Server struct {
	St        *store.Store
	DataDir   string
	BackupDir string
	DBPath    string
	Version   string
	tmpl      map[string]*template.Template
}

// NewServer builds the server and parses all templates.
func NewServer(st *store.Store, dataDir, backupDir, dbPath, version string) (*Server, error) {
	s := &Server{St: st, DataDir: dataDir, BackupDir: backupDir, DBPath: dbPath, Version: version}
	if err := s.parseTemplates(); err != nil {
		return nil, err
	}
	return s, nil
}

// Handler returns the routed http.Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	staticFS, _ := fs.Sub(assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheStatic(http.FileServer(http.FS(staticFS)))))

	// Dashboard and global views.
	mux.HandleFunc("GET /{$}", s.dashboard)
	mux.HandleFunc("GET /tasks", s.globalTasks)
	mux.HandleFunc("GET /risks", s.globalRisks)
	mux.HandleFunc("GET /decisions", s.globalDecisions)
	mux.HandleFunc("GET /benefits", s.globalBenefits)
	mux.HandleFunc("GET /reports", s.reports)
	mux.HandleFunc("GET /reports/project/{id}", s.projectReport)
	mux.HandleFunc("GET /projects/{id}/export/case", s.exportBusinessCase)
	mux.HandleFunc("GET /projects/{id}/export/status", s.exportStatusReport)
	mux.HandleFunc("GET /export/projects.csv", s.exportProjectsCSV)
	mux.HandleFunc("GET /export/projects.xlsx", s.exportProjectsXLSX)
	mux.HandleFunc("GET /lessons", s.globalLessons)

	// Ideas / portfolio pipeline.
	mux.HandleFunc("GET /ideas", s.ideas)
	mux.HandleFunc("POST /ideas", s.createIdea)
	mux.HandleFunc("POST /ideas/{id}/score", s.scoreIdea)
	mux.HandleFunc("POST /ideas/{id}/status", s.ideaStatus)
	mux.HandleFunc("POST /ideas/{id}/convert", s.convertIdea)
	mux.HandleFunc("POST /ideas/{id}/update", s.updateIdea)
	mux.HandleFunc("POST /ideas/{id}/delete", s.deleteIdea)

	// Projects.
	mux.HandleFunc("GET /projects", s.projects)
	mux.HandleFunc("GET /projects/new", s.newProjectForm)
	mux.HandleFunc("POST /projects", s.createProject)
	mux.HandleFunc("GET /projects/{id}", s.projectHome)
	mux.HandleFunc("GET /projects/{id}/{tab}", s.projectTab)
	mux.HandleFunc("POST /projects/{id}/update", s.updateProject)
	mux.HandleFunc("POST /projects/{id}/financials", s.updateFinancials)
	mux.HandleFunc("POST /projects/{id}/snapshots", s.captureStatusSnapshot)
	mux.HandleFunc("POST /projects/{id}/gate", s.advanceGate)
	mux.HandleFunc("POST /projects/{id}/close", s.closeProject)
	mux.HandleFunc("POST /projects/{id}/reopen", s.reopenProject)
	mux.HandleFunc("POST /projects/{id}/delete", s.deleteProject)
	mux.HandleFunc("POST /projects/{id}/documents/upload", s.uploadDocument)
	mux.HandleFunc("POST /projects/{id}/documents/link", s.addDocumentLink)
	mux.HandleFunc("GET /documents/{id}/download", s.downloadDocument)
	mux.HandleFunc("POST /documents/{id}/delete", s.deleteDocument)

	// Tasks & milestones.
	mux.HandleFunc("POST /projects/{id}/tasks", s.createTask)
	mux.HandleFunc("POST /tasks/{id}/update", s.updateTask)
	mux.HandleFunc("POST /tasks/{id}/status", s.taskStatus)
	mux.HandleFunc("POST /tasks/{id}/delete", s.deleteTask)
	mux.HandleFunc("POST /projects/{id}/milestones", s.createMilestone)
	mux.HandleFunc("POST /milestones/{id}/update", s.updateMilestone)
	mux.HandleFunc("POST /milestones/{id}/toggle", s.toggleMilestone)
	mux.HandleFunc("POST /milestones/{id}/delete", s.deleteMilestone)

	// RAID.
	mux.HandleFunc("POST /projects/{id}/raid", s.createRaid)
	mux.HandleFunc("POST /raid/{id}/update", s.updateRaid)
	mux.HandleFunc("POST /raid/{id}/status", s.raidStatus)
	mux.HandleFunc("POST /raid/{id}/delete", s.deleteRaid)

	// Decisions.
	mux.HandleFunc("POST /projects/{id}/decisions", s.createDecision)
	mux.HandleFunc("POST /decisions/{id}/record", s.recordDecision)
	mux.HandleFunc("POST /decisions/{id}/update", s.updateDecision)
	mux.HandleFunc("POST /decisions/{id}/delete", s.deleteDecision)

	// People: stakeholders + RACI.
	mux.HandleFunc("POST /projects/{id}/stakeholders", s.createStakeholder)
	mux.HandleFunc("POST /stakeholders/{id}/update", s.updateStakeholder)
	mux.HandleFunc("POST /stakeholders/{id}/delete", s.deleteStakeholder)
	mux.HandleFunc("POST /projects/{id}/raci", s.createRaciActivity)
	mux.HandleFunc("POST /raci/set", s.setRaci)
	mux.HandleFunc("POST /raci/{id}/delete", s.deleteRaciActivity)

	// Requirements, tests, change requests.
	mux.HandleFunc("POST /projects/{id}/scope", s.createScopeItem)
	mux.HandleFunc("POST /scope/{id}/update", s.updateScopeItem)
	mux.HandleFunc("POST /scope/{id}/delete", s.deleteScopeItem)
	mux.HandleFunc("POST /projects/{id}/scope/baselines", s.approveScopeBaseline)
	mux.HandleFunc("POST /projects/{id}/requirements", s.createRequirement)
	mux.HandleFunc("POST /requirements/{id}/update", s.updateRequirement)
	mux.HandleFunc("POST /requirements/{id}/delete", s.deleteRequirement)
	mux.HandleFunc("POST /projects/{id}/tests", s.createTest)
	mux.HandleFunc("POST /tests/{id}/status", s.testStatus)
	mux.HandleFunc("POST /tests/{id}/update", s.updateTest)
	mux.HandleFunc("POST /tests/{id}/delete", s.deleteTest)
	mux.HandleFunc("POST /projects/{id}/changes", s.createChange)
	mux.HandleFunc("POST /changes/{id}/update", s.updateChange)
	mux.HandleFunc("POST /changes/{id}/decide", s.decideChange)
	mux.HandleFunc("POST /changes/{id}/delete", s.deleteChange)

	// Discovery: pain points.
	mux.HandleFunc("POST /projects/{id}/painpoints", s.createPainPoint)
	mux.HandleFunc("POST /painpoints/{id}/delete", s.deletePainPoint)

	// Benefits.
	mux.HandleFunc("POST /projects/{id}/benefits", s.createBenefit)
	mux.HandleFunc("POST /benefits/{id}/update", s.updateBenefit)
	mux.HandleFunc("POST /benefits/{id}/measure", s.addMeasurement)
	mux.HandleFunc("POST /benefits/{id}/delete", s.deleteBenefit)
	mux.HandleFunc("POST /measurements/{id}/delete", s.deleteMeasurement)

	// Implementation readiness.
	mux.HandleFunc("POST /projects/{id}/readiness", s.createReadiness)
	mux.HandleFunc("POST /projects/{id}/readiness/seed", s.seedReadiness)
	mux.HandleFunc("POST /readiness/{id}/toggle", s.toggleReadiness)
	mux.HandleFunc("POST /readiness/{id}/delete", s.deleteReadiness)

	// Lessons learned.
	mux.HandleFunc("POST /projects/{id}/lessons", s.createLesson)
	mux.HandleFunc("POST /lessons/{id}/delete", s.deleteLesson)

	// Settings, backups, demo data.
	mux.HandleFunc("GET /settings", s.settings)
	mux.HandleFunc("POST /settings", s.saveSettings)
	mux.HandleFunc("POST /backup", s.makeBackup)
	mux.HandleFunc("GET /backup/download/{name}", s.downloadBackup)
	mux.HandleFunc("POST /restore", s.restoreBackup)
	mux.HandleFunc("POST /restore/upload", s.restoreUpload)
	mux.HandleFunc("POST /import/projects", s.importProjects)

	return mux
}

func cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}

// pathID extracts the {id} path segment as an int64.
func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// formInt parses an int form value with a default of 0.
func formInt(r *http.Request, key string) int {
	v, _ := strconv.Atoi(r.FormValue(key))
	return v
}

func formInt64(r *http.Request, key string) int64 {
	v, _ := strconv.ParseInt(r.FormValue(key), 10, 64)
	return v
}

func formInt64s(r *http.Request, key string) []int64 {
	_ = r.ParseForm()
	var out []int64
	for _, raw := range r.PostForm[key] {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil && v > 0 {
			out = append(out, v)
		}
	}
	return out
}

// formFloatPtr parses an optional float field; blank returns nil.
func formFloatPtr(r *http.Request, key string) *float64 {
	raw := r.FormValue(key)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &v
}

func formFloat(r *http.Request, key string) float64 {
	v, _ := strconv.ParseFloat(r.FormValue(key), 64)
	return v
}
