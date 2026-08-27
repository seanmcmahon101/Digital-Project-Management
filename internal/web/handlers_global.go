package web

import (
	"archive/zip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/coach"
	"github.com/seanmcmahon101/Digital-Project-Management/internal/db"
	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
)

// portfolioRow couples a project with its computed assessment for
// dashboards and reports.
type portfolioRow struct {
	P       store.Project
	Snap    *store.Snapshot
	Health  coach.Health
	Gate    coach.GateCheck
	Next    string
	Finance store.FinancialSummary
}

// loadPortfolio assesses every project.
func (s *Server) loadPortfolio() ([]portfolioRow, error) {
	projects, err := s.St.Projects()
	if err != nil {
		return nil, err
	}
	var rows []portfolioRow
	for _, p := range projects {
		snap, err := s.St.LoadSnapshot(p.ID)
		if err != nil {
			return nil, err
		}
		row := portfolioRow{P: p, Snap: snap, Health: coach.Assess(snap), Gate: coach.CheckGate(snap),
			Finance: store.SummariseFinancials(snap.Financials, snap.Benefits)}
		if a, ok := coach.NextAction(snap); ok {
			row.Next = a.Message
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	rows, err := s.loadPortfolio()
	if err != nil {
		s.serverError(w, err)
		return
	}
	ideas, err := s.St.Ideas()
	if err != nil {
		s.serverError(w, err)
		return
	}

	active, closed := 0, 0
	byStage := map[string]int{}
	healthCount := map[string]int{}
	var attention []portfolioRow
	var expectedAnnual, realisedAnnual, hoursSavedMonthly float64
	benefitsMeasured := 0
	for _, row := range rows {
		if row.P.IsClosed() {
			closed++
		} else {
			active++
			byStage[row.P.Stage]++
			healthCount[row.Health.Status]++
			if row.Health.Status != "green" {
				attention = append(attention, row)
			}
		}
		for _, b := range row.Snap.Benefits {
			expectedAnnual += b.AnnualValue
			realisedAnnual += b.RealisedAnnualValue()
			hoursSavedMonthly += b.MonthlyHoursSaved()
			if len(b.Measurements) > 0 {
				benefitsMeasured++
			}
		}
	}
	openIdeas := 0
	for _, i := range ideas {
		if i.Status == "new" || i.Status == "scored" || i.Status == "approved" {
			openIdeas++
		}
	}

	milestones, err := s.St.UpcomingMilestones(21)
	if err != nil {
		s.serverError(w, err)
		return
	}
	openTasks, err := s.St.OpenTasksAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	var overdue []store.Task
	for _, t := range openTasks {
		if t.Overdue() {
			overdue = append(overdue, t)
		}
	}
	risks, err := s.St.OpenRisksAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	var majorRisks []store.RaidItem
	for _, rk := range risks {
		if rk.Severity() == "high" {
			majorRisks = append(majorRisks, rk)
		}
	}
	if len(majorRisks) > 6 {
		majorRisks = majorRisks[:6]
	}

	// "What should I do next": the single top action per active project.
	type nextItem struct {
		P      store.Project
		Advice coach.Advice
	}
	var nextActions []nextItem
	for _, row := range rows {
		if row.P.IsClosed() {
			continue
		}
		if a, ok := coach.NextAction(row.Snap); ok {
			nextActions = append(nextActions, nextItem{P: row.P, Advice: a})
		}
	}
	// Most urgent first.
	rank := map[string]int{"act": 0, "soon": 1, "consider": 2}
	for i := 1; i < len(nextActions); i++ {
		for j := i; j > 0 && rank[nextActions[j].Advice.Severity] < rank[nextActions[j-1].Advice.Severity]; j-- {
			nextActions[j], nextActions[j-1] = nextActions[j-1], nextActions[j]
		}
	}

	maxStage := 1
	for _, n := range byStage {
		if n > maxStage {
			maxStage = n
		}
	}

	s.render(w, r, "dashboard", view{
		Title: "Dashboard", Active: "dashboard",
		Data: map[string]any{
			"Rows": rows, "Active": active, "Closed": closed, "OpenIdeas": openIdeas,
			"ByStage": byStage, "MaxStage": maxStage, "HealthCount": healthCount,
			"Attention": attention, "Milestones": milestones, "Overdue": overdue,
			"MajorRisks": majorRisks, "NextActions": nextActions,
			"ExpectedAnnual": expectedAnnual, "RealisedAnnual": realisedAnnual,
			"HoursSavedMonthly": hoursSavedMonthly, "BenefitsMeasured": benefitsMeasured,
			"HasAnything": len(rows) > 0 || len(ideas) > 0,
		},
	})
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if runes := []rune(query); len(runes) > 100 {
		query = string(runes[:100])
	}
	results, err := s.St.Search(query, 50)
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "search", view{Title: "Search", Active: "search",
		Data: map[string]any{"Query": query, "Results": results}})
}

// --- Global lists ---

func (s *Server) globalTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.St.OpenTasksAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "tasks", view{Title: "Work", Active: "tasks",
		Data: map[string]any{"Tasks": tasks}})
}

func (s *Server) globalRisks(w http.ResponseWriter, r *http.Request) {
	risks, err := s.St.OpenRisksAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "risks", view{Title: "Risks & issues", Active: "risks",
		Data: map[string]any{"Risks": risks}})
}

func (s *Server) globalDecisions(w http.ResponseWriter, r *http.Request) {
	decisions, err := s.St.DecisionsAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "decisions", view{Title: "Decisions", Active: "decisions",
		Data: map[string]any{"Decisions": decisions}})
}

func (s *Server) globalBenefits(w http.ResponseWriter, r *http.Request) {
	benefits, err := s.St.BenefitsAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	var expected, realised, hoursMonthly float64
	achieved := 0
	for _, b := range benefits {
		expected += b.AnnualValue
		realised += b.RealisedAnnualValue()
		hoursMonthly += b.MonthlyHoursSaved()
		if b.Achieved() {
			achieved++
		}
	}
	s.render(w, r, "benefits", view{Title: "Benefits", Active: "benefits",
		Data: map[string]any{
			"Benefits": benefits, "Expected": expected, "Realised": realised,
			"HoursMonthly": hoursMonthly, "Achieved": achieved,
		}})
}

func (s *Server) globalLessons(w http.ResponseWriter, r *http.Request) {
	lessons, err := s.St.LessonsAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "lessons", view{Title: "Lessons learned", Active: "lessons",
		Data: map[string]any{"Lessons": lessons}})
}

// --- Reports ---

func (s *Server) reports(w http.ResponseWriter, r *http.Request) {
	rows, err := s.loadPortfolio()
	if err != nil {
		s.serverError(w, err)
		return
	}
	benefits, err := s.St.BenefitsAllProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	var expected, realised float64
	var investment, approved, actual float64
	for _, b := range benefits {
		expected += b.AnnualValue
		realised += b.RealisedAnnualValue()
	}
	for _, row := range rows {
		investment += row.Finance.Investment
		approved += row.Finance.Financials.ApprovedBudget
		actual += row.Finance.Financials.ActualCost
	}
	portfolioROI, hasPortfolioROI := 0.0, false
	if investment > 0 {
		portfolioROI = (expected - investment) / investment * 100
		hasPortfolioROI = true
	}
	s.render(w, r, "reports", view{Title: "Reports", Active: "reports",
		Data: map[string]any{
			"Rows": rows, "Benefits": benefits,
			"Expected": expected, "Realised": realised,
			"Investment": investment, "ApprovedBudget": approved, "ActualCost": actual,
			"PortfolioROI": portfolioROI, "HasPortfolioROI": hasPortfolioROI,
			"Today": store.Today(),
		}})
}

func (s *Server) projectReport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	data, err := s.projectView(id)
	if err != nil {
		s.notFound(w)
		return
	}
	hist, err := s.St.GateHistory(id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	data["GateHistory"] = hist
	statusHist, err := s.St.StatusHistory(id, 12)
	if err != nil {
		s.serverError(w, err)
		return
	}
	data["StatusHistory"] = statusHist
	data["Today"] = store.Today()
	p := data["P"].(store.Project)
	s.render(w, r, "report_project", view{Title: p.Code + " status report", Active: "reports", Data: data})
}

// --- Settings, backup, restore, demo ---

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "settings", view{Title: "Settings", Active: "settings",
		Data: map[string]any{
			"OrgName":      s.St.Setting("org_name", ""),
			"CurrencyV":    s.St.Setting("currency", "£"),
			"HourlyRate":   s.St.Setting("hourly_rate", "30"),
			"SidebarColor": normaliseHexColor(s.St.Setting("sidebar_color", "#5C1E30")),
			"Backups":      store.ListBackups(s.BackupDir),
			"DataDir":      s.DataDir,
			"Version":      s.Version,
		}})
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	sidebarColor := strings.TrimSpace(r.FormValue("sidebar_color"))
	if normaliseHexColor(sidebarColor) != strings.ToUpper(sidebarColor) {
		s.fail(w, r, "/settings", errValidation("Choose a valid six-digit sidebar colour"))
		return
	}
	pairs := map[string]string{
		"org_name":      strings.TrimSpace(r.FormValue("org_name")),
		"currency":      strings.TrimSpace(r.FormValue("currency")),
		"hourly_rate":   strings.TrimSpace(r.FormValue("hourly_rate")),
		"sidebar_color": strings.ToUpper(sidebarColor),
	}
	if len([]rune(pairs["org_name"])) > 120 {
		s.fail(w, r, "/settings", errValidation("Organisation name must be 120 characters or fewer"))
		return
	}
	if pairs["currency"] == "" {
		pairs["currency"] = "£"
	}
	if len([]rune(pairs["currency"])) > 3 {
		s.fail(w, r, "/settings", errValidation("Currency symbol must be 3 characters or fewer"))
		return
	}
	hourlyRate, err := strconv.ParseFloat(pairs["hourly_rate"], 64)
	if err != nil || math.IsNaN(hourlyRate) || math.IsInf(hourlyRate, 0) || hourlyRate < 0 {
		s.fail(w, r, "/settings", errValidation("Hourly rate must be a non-negative number"))
		return
	}
	if err := s.St.SetSettings(pairs); err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	s.redirect(w, r, "/settings", "Settings saved.")
}

func (s *Server) makeBackup(w http.ResponseWriter, r *http.Request) {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()
	backupPath, err := s.St.BackupWorkspace(s.DataDir, s.BackupDir, s.Version)
	if err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	s.redirect(w, r, "/settings", "Complete workspace backup created: "+filepath.Base(backupPath))
}

func (s *Server) downloadBackup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	backupPath, ok := existingBackupPath(s.BackupDir, name)
	if !ok {
		s.notFound(w)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, backupPath)
}

// restoreBackup restores from an existing backup in the backups folder.
func (s *Server) restoreBackup(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	backupPath, ok := existingBackupPath(s.BackupDir, name)
	if !ok {
		s.fail(w, r, "/settings", errValidation("Choose a backup to restore"))
		return
	}
	s.doRestore(w, r, backupPath)
}

func existingBackupPath(dir, name string) (string, bool) {
	if !store.IsBackupName(name) {
		return "", false
	}
	backupPath := filepath.Join(dir, name)
	info, err := os.Lstat(backupPath)
	return backupPath, err == nil && info.Mode().IsRegular()
}

const (
	maxRestoreUploadSize     int64 = 3 << 30 // bounded above archive payload limit
	maxRestoredWorkspaceSize       = store.WorkspaceBackupMaxSize
	maxRestoreFiles                = store.WorkspaceBackupMaxFiles
	maxManifestSize          int64 = 1 << 20 // 1 MiB
)

// restoreUpload accepts current full-workspace archives and legacy database
// backups. Both the HTTP body and copied file are explicitly bounded.
func (s *Server) restoreUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRestoreUploadSize+(1<<20))
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		s.fail(w, r, "/settings", errValidation("Choose a backup file no larger than 3 GB"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		s.fail(w, r, "/settings", errValidation("Choose a .dpm-backup or legacy .db file to restore"))
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	if ext != store.WorkspaceBackupExtension && ext != ".db" {
		s.fail(w, r, "/settings", errValidation("Choose a .dpm-backup or legacy .db file to restore"))
		return
	}
	out, err := os.CreateTemp(s.DataDir, ".restore-upload-*.tmp")
	if err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	tmp := out.Name()
	n, copyErr := io.Copy(out, io.LimitReader(file, maxRestoreUploadSize+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || n > maxRestoreUploadSize {
		os.Remove(tmp)
		if n > maxRestoreUploadSize {
			s.fail(w, r, "/settings", errValidation("Backup files must be 3 GB or smaller"))
		} else if copyErr != nil {
			s.fail(w, r, "/settings", copyErr)
		} else {
			s.fail(w, r, "/settings", closeErr)
		}
		return
	}
	if n == 0 {
		os.Remove(tmp)
		s.fail(w, r, "/settings", errValidation("The selected backup file is empty"))
		return
	}
	defer os.Remove(tmp)
	s.doRestore(w, r, tmp)
}

// doRestore validates and stages the complete candidate before taking a full
// safety backup and replacing live data. Full archives replace both SQLite and
// uploads; legacy .db restores deliberately retain the current uploads folder.
func (s *Server) doRestore(w http.ResponseWriter, r *http.Request, candidate string) {
	stageDir, err := os.MkdirTemp(s.DataDir, ".restore-stage-")
	if err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	defer os.RemoveAll(stageDir)

	stagedDB := filepath.Join(stageDir, "app.db")
	stagedUploads := filepath.Join(stageDir, "uploads")
	fullWorkspace, err := looksLikeZip(candidate)
	if err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	if fullWorkspace {
		if err := extractWorkspaceBackup(candidate, stageDir); err != nil {
			s.fail(w, r, "/settings", errValidation("That workspace backup is invalid or damaged: "+err.Error()))
			return
		}
	} else {
		if err := copyFileBounded(candidate, stagedDB, maxRestoreUploadSize); err != nil {
			s.fail(w, r, "/settings", errValidation("That legacy database backup is invalid: "+err.Error()))
			return
		}
	}
	if err := validateRestoreDatabase(stagedDB); err != nil {
		s.fail(w, r, "/settings", errValidation("That file is not a valid backup of this application"))
		return
	}
	if fullWorkspace {
		if err := os.MkdirAll(stagedUploads, 0o750); err != nil {
			s.fail(w, r, "/settings", err)
			return
		}
	}

	if _, err := s.St.BackupWorkspace(s.DataDir, s.BackupDir, s.Version); err != nil {
		s.fail(w, r, "/settings", fmt.Errorf("safety backup failed, restore aborted: %w", err))
		return
	}
	if err := s.installStagedRestore(stagedDB, stagedUploads, fullWorkspace); err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	message := "Workspace restored. A complete safety backup of the previous data was kept."
	if !fullWorkspace {
		message = "Legacy database restored; current uploaded files were retained. A complete safety backup was kept."
	}
	s.redirect(w, r, "/", message)
}

func looksLikeZip(filename string) (bool, error) {
	f, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var signature [4]byte
	if _, err := io.ReadFull(f, signature[:]); err != nil {
		return false, err
	}
	return signature == [4]byte{'P', 'K', 3, 4} ||
		signature == [4]byte{'P', 'K', 5, 6} ||
		signature == [4]byte{'P', 'K', 7, 8}, nil
}

func extractWorkspaceBackup(filename, stageDir string) error {
	zr, err := zip.OpenReader(filename)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer zr.Close()
	if len(zr.File) == 0 || len(zr.File) > maxRestoreFiles+1 {
		return fmt.Errorf("archive contains an invalid number of files")
	}

	var manifestFile *zip.File
	archiveFiles := make(map[string]*zip.File, len(zr.File))
	for _, file := range zr.File {
		if _, exists := archiveFiles[file.Name]; exists {
			return fmt.Errorf("archive contains duplicate path %q", file.Name)
		}
		archiveFiles[file.Name] = file
		if file.Name == "manifest.json" {
			manifestFile = file
		}
	}
	if manifestFile == nil || manifestFile.UncompressedSize64 > uint64(maxManifestSize) {
		return fmt.Errorf("manifest is missing or too large")
	}
	manifestBody, err := readZipFileBounded(manifestFile, maxManifestSize)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	var manifest store.BackupManifest
	decoder := json.NewDecoder(strings.NewReader(string(manifestBody)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("manifest contains trailing data")
	}
	if manifest.Format != store.WorkspaceBackupFormat || manifest.Version != store.WorkspaceBackupVersion {
		return fmt.Errorf("unsupported backup format or version")
	}
	if len(manifest.Files) == 0 || len(manifest.Files) > maxRestoreFiles {
		return fmt.Errorf("manifest contains an invalid number of files")
	}

	declared := make(map[string]store.BackupManifestFile, len(manifest.Files))
	var total int64
	for _, entry := range manifest.Files {
		if !safeWorkspaceArchivePath(entry.Path) || entry.Size < 0 || entry.Size > maxRestoredWorkspaceSize {
			return fmt.Errorf("manifest contains invalid path or size %q", entry.Path)
		}
		if _, exists := declared[entry.Path]; exists {
			return fmt.Errorf("manifest contains duplicate path %q", entry.Path)
		}
		checksum, err := hex.DecodeString(entry.SHA256)
		if err != nil || len(checksum) != sha256.Size {
			return fmt.Errorf("manifest contains an invalid checksum for %q", entry.Path)
		}
		total += entry.Size
		if total > maxRestoredWorkspaceSize {
			return fmt.Errorf("restored workspace would exceed 2 GB")
		}
		declared[entry.Path] = entry
	}
	if _, ok := declared["app.db"]; !ok {
		return fmt.Errorf("database is missing from manifest")
	}
	if len(archiveFiles) != len(declared)+1 {
		return fmt.Errorf("archive contains files not declared by its manifest")
	}

	for archivePath, entry := range declared {
		file, ok := archiveFiles[archivePath]
		if !ok {
			return fmt.Errorf("archive is missing %q", archivePath)
		}
		if file.UncompressedSize64 > uint64(maxRestoredWorkspaceSize) || file.FileInfo().IsDir() ||
			!file.Mode().IsRegular() || int64(file.UncompressedSize64) != entry.Size {
			return fmt.Errorf("archive metadata is invalid for %q", archivePath)
		}
		destination := filepath.Join(stageDir, filepath.FromSlash(archivePath))
		if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		mode := os.FileMode(0o640)
		if archivePath == "app.db" {
			mode = 0o600
		}
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			in.Close()
			return err
		}
		hash := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(in, entry.Size+1))
		inErr := in.Close()
		outErr := out.Close()
		if copyErr != nil || inErr != nil || outErr != nil {
			return fmt.Errorf("extract %q: %v %v %v", archivePath, copyErr, inErr, outErr)
		}
		if n != entry.Size || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), entry.SHA256) {
			return fmt.Errorf("size or checksum mismatch for %q", archivePath)
		}
	}
	return nil
}

func safeWorkspaceArchivePath(name string) bool {
	if name == "" || strings.ContainsRune(name, 0) || strings.Contains(name, "\\") || path.IsAbs(name) || path.Clean(name) != name {
		return false
	}
	return name == "app.db" || (strings.HasPrefix(name, "uploads/") && len(name) > len("uploads/"))
}

func readZipFileBounded(file *zip.File, max int64) ([]byte, error) {
	in, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer in.Close()
	body, err := io.ReadAll(io.LimitReader(in, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > max {
		return nil, fmt.Errorf("file exceeds size limit")
	}
	return body, nil
}

func validateRestoreDatabase(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	var header [16]byte
	_, headerErr := io.ReadFull(file, header[:])
	file.Close()
	if headerErr != nil || string(header[:]) != "SQLite format 3\x00" {
		return fmt.Errorf("file is not a SQLite database")
	}
	// Check product identity before db.Open applies migrations. This prevents a
	// valid but unrelated/empty SQLite database being silently accepted and
	// converted into a blank Digital Project Management workspace.
	raw, err := sql.Open("sqlite", "file:"+filename+"?mode=ro")
	if err != nil {
		return err
	}
	var identityTables int
	identityErr := raw.QueryRow(`SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name IN ('projects', 'schema_migrations')`).Scan(&identityTables)
	raw.Close()
	if identityErr != nil || identityTables != 2 {
		return fmt.Errorf("database does not contain the Digital Project Management schema")
	}

	check, err := db.Open(filename)
	if err != nil {
		return err
	}
	var count int
	queryErr := check.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&count)
	if queryErr == nil {
		_, queryErr = check.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	}
	closeErr := check.Close()
	if queryErr != nil {
		return queryErr
	}
	return closeErr
}

func (s *Server) installStagedRestore(stagedDB, stagedUploads string, replaceUploads bool) error {
	rollbackDir, err := os.MkdirTemp(s.DataDir, ".restore-rollback-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(rollbackDir)
	rollbackDB := filepath.Join(rollbackDir, "app.db")
	rollbackUploads := filepath.Join(rollbackDir, "uploads")
	liveUploads := filepath.Join(s.DataDir, "uploads")

	if err := s.St.DB.Close(); err != nil {
		return fmt.Errorf("close current database: %w", err)
	}
	os.Remove(s.DBPath + "-wal")
	os.Remove(s.DBPath + "-shm")

	reopenCurrent := func() {
		if reopened, reopenErr := db.Open(s.DBPath); reopenErr == nil {
			s.St.DB = reopened
		}
	}
	if err := os.Rename(s.DBPath, rollbackDB); err != nil {
		reopenCurrent()
		return fmt.Errorf("preserve current database: %w", err)
	}
	hadUploads := false
	if replaceUploads {
		if _, err := os.Stat(liveUploads); err == nil {
			hadUploads = true
			if err := os.Rename(liveUploads, rollbackUploads); err != nil {
				_ = os.Rename(rollbackDB, s.DBPath)
				reopenCurrent()
				return fmt.Errorf("preserve current uploads: %w", err)
			}
		} else if !os.IsNotExist(err) {
			_ = os.Rename(rollbackDB, s.DBPath)
			reopenCurrent()
			return fmt.Errorf("inspect current uploads: %w", err)
		}
	}

	dbInstalled, uploadsInstalled := false, false
	rollback := func(cause error) error {
		if dbInstalled {
			_ = os.Remove(s.DBPath)
		}
		if uploadsInstalled {
			_ = os.RemoveAll(liveUploads)
		}
		_ = os.Rename(rollbackDB, s.DBPath)
		if hadUploads {
			_ = os.Rename(rollbackUploads, liveUploads)
		}
		reopened, reopenErr := db.Open(s.DBPath)
		if reopenErr == nil {
			s.St.DB = reopened
			return cause
		}
		return fmt.Errorf("%w; additionally failed to reopen previous database: %v", cause, reopenErr)
	}

	if err := os.Rename(stagedDB, s.DBPath); err != nil {
		return rollback(fmt.Errorf("install restored database: %w", err))
	}
	dbInstalled = true
	if err := os.Chmod(s.DBPath, 0o600); err != nil {
		return rollback(fmt.Errorf("secure restored database: %w", err))
	}
	if replaceUploads {
		if err := os.Rename(stagedUploads, liveUploads); err != nil {
			return rollback(fmt.Errorf("install restored uploads: %w", err))
		}
		uploadsInstalled = true
	}
	reopened, err := db.Open(s.DBPath)
	if err != nil {
		return rollback(fmt.Errorf("open restored database: %w", err))
	}
	s.St.DB = reopened
	return nil
}

func copyFileBounded(src, dst string, max int64) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(out, io.LimitReader(in, max+1))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n > max {
		return fmt.Errorf("file exceeds size limit")
	}
	return nil
}
