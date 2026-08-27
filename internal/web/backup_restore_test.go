package web

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
)

func TestCompleteWorkspaceBackupRestoreReplacesDatabaseAndUploads(t *testing.T) {
	srv, st := testHTTPServer(t)
	project, err := st.CreateProject("Before backup", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	uploadPath := filepath.Join(srv.DataDir, "uploads", itoa64(project.ID), "evidence.txt")
	if err := os.MkdirAll(filepath.Dir(uploadPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(uploadPath, []byte("original document"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := st.AddFileDocument(project.ID, "Evidence", "", "evidence.txt", "evidence.txt", "text/plain", 17); err != nil {
		t.Fatal(err)
	}
	archivePath, err := st.BackupWorkspace(srv.DataDir, srv.BackupDir, srv.Version)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateProjectFields(project.ID, map[string]string{"name": "After backup"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(uploadPath, []byte("changed document"), 0o640); err != nil {
		t.Fatal(err)
	}

	rr := performRequest(srv.Handler(), http.MethodPost, "/restore", url.Values{
		"name": {filepath.Base(archivePath)},
	})
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/" {
		t.Fatalf("restore returned %d location=%q body=%s", rr.Code, rr.Header().Get("Location"), rr.Body.String())
	}
	t.Cleanup(func() { srv.St.DB.Close() })
	restored, err := st.Project(project.ID)
	if err != nil || restored.Name != "Before backup" {
		t.Fatalf("restored project = %+v, err=%v", restored, err)
	}
	body, err := os.ReadFile(uploadPath)
	if err != nil || string(body) != "original document" {
		t.Fatalf("restored upload = %q, err=%v", body, err)
	}
	if backups := store.ListBackups(srv.BackupDir); len(backups) < 2 {
		t.Fatalf("restore did not retain a safety backup: %+v", backups)
	}
}

func TestLegacyDatabaseRestoreRetainsCurrentUploads(t *testing.T) {
	srv, st := testHTTPServer(t)
	project, err := st.CreateProject("Legacy state", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	legacyPath, err := st.Backup(srv.BackupDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateProjectFields(project.ID, map[string]string{"name": "Current state"}); err != nil {
		t.Fatal(err)
	}
	uploadPath := filepath.Join(srv.DataDir, "uploads", itoa64(project.ID), "current.txt")
	if err := os.MkdirAll(filepath.Dir(uploadPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(uploadPath, []byte("retain this upload"), 0o640); err != nil {
		t.Fatal(err)
	}

	rr := performRequest(srv.Handler(), http.MethodPost, "/restore", url.Values{
		"name": {filepath.Base(legacyPath)},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("legacy restore returned %d: %s", rr.Code, rr.Body.String())
	}
	t.Cleanup(func() { srv.St.DB.Close() })
	restored, err := st.Project(project.ID)
	if err != nil || restored.Name != "Legacy state" {
		t.Fatalf("legacy restored project = %+v, err=%v", restored, err)
	}
	if body, err := os.ReadFile(uploadPath); err != nil || string(body) != "retain this upload" {
		t.Fatalf("legacy restore changed uploads: %q, err=%v", body, err)
	}
}

func TestWorkspaceArchiveValidationRejectsChecksumMismatchAndTraversal(t *testing.T) {
	validDB := []byte("not needed for structural validation")
	wrongHash := sha256.Sum256([]byte("different"))
	archive := writeTestWorkspaceArchive(t, []store.BackupManifestFile{{
		Path: "app.db", Size: int64(len(validDB)), SHA256: hex.EncodeToString(wrongHash[:]),
	}}, map[string][]byte{"app.db": validDB})
	if err := extractWorkspaceBackup(archive, t.TempDir()); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("checksum mismatch error = %v", err)
	}

	body := []byte("escape")
	hash := sha256.Sum256(body)
	traversal := writeTestWorkspaceArchive(t, []store.BackupManifestFile{{
		Path: "uploads/../../escape", Size: int64(len(body)), SHA256: hex.EncodeToString(hash[:]),
	}}, map[string][]byte{"uploads/../../escape": body})
	if err := extractWorkspaceBackup(traversal, t.TempDir()); err == nil || !strings.Contains(err.Error(), "invalid path") {
		t.Fatalf("traversal error = %v", err)
	}
}

func TestInvalidWorkspaceRestoreLeavesLiveDataUntouched(t *testing.T) {
	srv, st := testHTTPServer(t)
	project, err := st.CreateProject("Keep live workspace", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	invalidDB := []byte("this is not sqlite")
	hash := sha256.Sum256(invalidDB)
	archive := writeTestWorkspaceArchive(t, []store.BackupManifestFile{{
		Path: "app.db", Size: int64(len(invalidDB)), SHA256: hex.EncodeToString(hash[:]),
	}}, map[string][]byte{"app.db": invalidDB})

	rr := httptest.NewRecorder()
	srv.doRestore(rr, httptest.NewRequest(http.MethodPost, "/restore", nil), archive)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/settings" {
		t.Fatalf("invalid restore returned %d location=%q", rr.Code, rr.Header().Get("Location"))
	}
	stillLive, err := st.Project(project.ID)
	if err != nil || stillLive.Name != "Keep live workspace" {
		t.Fatalf("live workspace changed after invalid restore: %+v err=%v", stillLive, err)
	}
	if backups := store.ListBackups(srv.BackupDir); len(backups) != 0 {
		t.Fatalf("invalid restore should fail before taking a safety backup: %+v", backups)
	}
}

func TestDeleteProjectRemovesUploadedFiles(t *testing.T) {
	srv, st := testHTTPServer(t)
	project, err := st.CreateProject("Delete uploads", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	uploadDir := filepath.Join(srv.DataDir, "uploads", itoa64(project.ID))
	if err := os.MkdirAll(uploadDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploadDir, "orphan.txt"), []byte("remove me"), 0o640); err != nil {
		t.Fatal(err)
	}
	rr := performRequest(srv.Handler(), http.MethodPost, "/projects/"+itoa64(project.ID)+"/delete", url.Values{
		"confirm_code": {project.Code},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("delete project returned %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(uploadDir); !os.IsNotExist(err) {
		t.Fatalf("project upload directory still exists: %v", err)
	}
}

func writeTestWorkspaceArchive(t *testing.T, manifestFiles []store.BackupManifestFile, files map[string][]byte) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "test"+store.WorkspaceBackupExtension)
	out, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	for name, body := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o640)
		entry, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	manifest := store.BackupManifest{
		Format: store.WorkspaceBackupFormat, Version: store.WorkspaceBackupVersion,
		CreatedAt: "2026-08-27T12:00:00Z", ApplicationVersion: "test", Files: manifestFiles,
	}
	manifestBody, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := zw.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(manifestBody); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return filename
}
