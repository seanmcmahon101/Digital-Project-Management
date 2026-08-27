package store

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/db"
)

func TestBackupWorkspaceContainsDatabaseUploadsAndVerifiedManifest(t *testing.T) {
	dataDir := t.TempDir()
	databasePath := filepath.Join(dataDir, "app.db")
	sqldb, err := db.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer sqldb.Close()
	s := New(sqldb)
	project, err := s.CreateProject("Archive me", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	uploadPath := filepath.Join(dataDir, "uploads", "1", "evidence.pdf")
	if err := os.MkdirAll(filepath.Dir(uploadPath), 0o750); err != nil {
		t.Fatal(err)
	}
	uploadBody := []byte("important uploaded evidence")
	if err := os.WriteFile(uploadPath, uploadBody, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := s.AddFileDocument(project.ID, "Evidence", "", "evidence.pdf", "evidence.pdf", "application/pdf", int64(len(uploadBody))); err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(dataDir, "backups")
	archivePath, err := s.BackupWorkspace(dataDir, backupDir, "9.8.7")
	if err != nil {
		t.Fatalf("BackupWorkspace: %v", err)
	}
	if !strings.HasSuffix(archivePath, WorkspaceBackupExtension) {
		t.Fatalf("backup path = %q", archivePath)
	}

	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	for _, file := range zr.File {
		files[file.Name] = file
	}
	for _, name := range []string{"app.db", "uploads/1/evidence.pdf", "manifest.json"} {
		if files[name] == nil {
			t.Errorf("archive missing %s", name)
		}
	}

	manifestReader, err := files["manifest.json"].Open()
	if err != nil {
		t.Fatal(err)
	}
	var manifest BackupManifest
	if err := json.NewDecoder(manifestReader).Decode(&manifest); err != nil {
		manifestReader.Close()
		t.Fatal(err)
	}
	manifestReader.Close()
	if manifest.Format != WorkspaceBackupFormat || manifest.Version != WorkspaceBackupVersion || manifest.ApplicationVersion != "9.8.7" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if len(manifest.Files) != 2 {
		t.Fatalf("manifest files = %d, want 2", len(manifest.Files))
	}
	for _, entry := range manifest.Files {
		file := files[entry.Path]
		if file == nil {
			t.Fatalf("manifest entry %q is not in archive", entry.Path)
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.New()
		n, err := io.Copy(hash, reader)
		reader.Close()
		if err != nil || n != entry.Size || hex.EncodeToString(hash.Sum(nil)) != entry.SHA256 {
			t.Fatalf("manifest verification failed for %s: size=%d err=%v", entry.Path, n, err)
		}
	}

	extractedDB := filepath.Join(t.TempDir(), "restored.db")
	dbReader, err := files["app.db"].Open()
	if err != nil {
		t.Fatal(err)
	}
	dbOut, err := os.Create(extractedDB)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(dbOut, dbReader)
	dbReader.Close()
	dbOut.Close()
	if copyErr != nil {
		t.Fatal(copyErr)
	}
	restoredDB, err := db.Open(extractedDB)
	if err != nil {
		t.Fatal(err)
	}
	defer restoredDB.Close()
	var projectName string
	if err := restoredDB.QueryRow(`SELECT name FROM projects`).Scan(&projectName); err != nil || projectName != "Archive me" {
		t.Fatalf("restored project = %q, err=%v", projectName, err)
	}
}

func TestAutoBackupWorkspaceIsDailyAndListsLegacyBackups(t *testing.T) {
	dataDir := t.TempDir()
	sqldb, err := db.Open(filepath.Join(dataDir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqldb.Close()
	s := New(sqldb)
	backupDir := filepath.Join(dataDir, "backups")
	first, err := s.AutoBackupWorkspace(dataDir, backupDir, "test", 14)
	if err != nil || first == "" {
		t.Fatalf("first workspace backup = %q, err=%v", first, err)
	}
	second, err := s.AutoBackupWorkspace(dataDir, backupDir, "test", 14)
	if err != nil || second != "" {
		t.Fatalf("same-day workspace backup = %q, err=%v", second, err)
	}
	if _, err := s.Backup(backupDir); err != nil {
		t.Fatal(err)
	}
	backups := ListBackups(backupDir)
	if len(backups) != 2 {
		t.Fatalf("ListBackups returned %d items", len(backups))
	}
	workspace, legacy := 0, 0
	for _, backup := range backups {
		if backup.IsWorkspace {
			workspace++
		} else {
			legacy++
		}
	}
	if workspace != 1 || legacy != 1 {
		t.Fatalf("workspace=%d legacy=%d", workspace, legacy)
	}
	for _, unsafe := range []string{"../backup-bad.db", "backup-bad.zip", "not-a-backup.db", "backup-dir/file.db"} {
		if IsBackupName(unsafe) {
			t.Errorf("IsBackupName accepted %q", unsafe)
		}
	}
}
