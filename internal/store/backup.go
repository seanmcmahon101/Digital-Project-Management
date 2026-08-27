package store

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// WorkspaceBackupExtension is deliberately distinct from .zip so users do
	// not accidentally edit the archive. The file itself is a standard ZIP.
	WorkspaceBackupExtension = ".dpm-backup"
	WorkspaceBackupFormat    = "digitalisationpm-workspace"
	WorkspaceBackupVersion   = 1
	WorkspaceBackupMaxFiles  = 10_000
	WorkspaceBackupMaxSize   = int64(2 << 30) // 2 GiB uncompressed
)

// BackupManifest is stored as manifest.json in every full workspace backup.
// Each payload file is checksummed so a truncated or modified archive is
// rejected before it can replace live data.
type BackupManifest struct {
	Format             string               `json:"format"`
	Version            int                  `json:"version"`
	CreatedAt          string               `json:"created_at"`
	ApplicationVersion string               `json:"application_version"`
	Files              []BackupManifestFile `json:"files"`
}

type BackupManifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Backup writes a legacy database-only backup. It remains available so old
// integrations and tests keep working; user-facing backups use BackupWorkspace.
func (s *Store) Backup(dir string) (string, error) {
	if err := ensurePrivateBackupDir(dir); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	path, err := uniqueBackupPath(dir, ".db")
	if err != nil {
		return "", err
	}
	// VACUUM INTO refuses to overwrite and produces a transactionally
	// consistent standalone database even while the live database uses WAL.
	if _, err := s.DB.Exec(`VACUUM INTO ?`, path); err != nil {
		return "", fmt.Errorf("backup database: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("secure backup database: %w", err)
	}
	return path, nil
}

// BackupWorkspace creates a versioned, self-validating archive containing a
// consistent SQLite snapshot plus every uploaded document.
func (s *Store) BackupWorkspace(dataDir, dir, appVersion string) (string, error) {
	if err := ensurePrivateBackupDir(dir); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	dbSnapshot, err := unusedTempPath(dir, ".backup-database-*.db")
	if err != nil {
		return "", err
	}
	defer os.Remove(dbSnapshot)
	if _, err := s.DB.Exec(`VACUUM INTO ?`, dbSnapshot); err != nil {
		return "", fmt.Errorf("snapshot database: %w", err)
	}

	archive, err := os.CreateTemp(dir, ".workspace-backup-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create backup archive: %w", err)
	}
	tmpArchive := archive.Name()
	committed := false
	defer func() {
		if !committed {
			archive.Close()
			os.Remove(tmpArchive)
		}
	}()

	zw := zip.NewWriter(archive)
	manifest := BackupManifest{
		Format:             WorkspaceBackupFormat,
		Version:            WorkspaceBackupVersion,
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		ApplicationVersion: appVersion,
	}
	entry, err := addFileToArchive(zw, dbSnapshot, "app.db")
	if err != nil {
		return "", fmt.Errorf("archive database: %w", err)
	}
	manifest.Files = append(manifest.Files, entry)
	totalSize := entry.Size
	if totalSize > WorkspaceBackupMaxSize {
		return "", fmt.Errorf("workspace exceeds the 2 GB backup limit")
	}

	uploadsDir := filepath.Join(dataDir, "uploads")
	if _, statErr := os.Stat(uploadsDir); statErr == nil {
		err = filepath.WalkDir(uploadsDir, func(filePath string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("uploaded document %q is not a regular file", filePath)
			}
			rel, err := filepath.Rel(dataDir, filePath)
			if err != nil {
				return err
			}
			archivePath := filepath.ToSlash(rel)
			entry, err := addFileToArchive(zw, filePath, archivePath)
			if err != nil {
				return err
			}
			manifest.Files = append(manifest.Files, entry)
			totalSize += entry.Size
			if len(manifest.Files) > WorkspaceBackupMaxFiles {
				return fmt.Errorf("workspace exceeds the %d-file backup limit", WorkspaceBackupMaxFiles)
			}
			if totalSize > WorkspaceBackupMaxSize {
				return fmt.Errorf("workspace exceeds the 2 GB backup limit")
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("archive uploaded documents: %w", err)
		}
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("inspect uploads: %w", statErr)
	}

	manifestBody, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode backup manifest: %w", err)
	}
	mw, err := zw.CreateHeader(&zip.FileHeader{Name: "manifest.json", Method: zip.Deflate})
	if err != nil {
		return "", fmt.Errorf("create backup manifest: %w", err)
	}
	if _, err := mw.Write(manifestBody); err != nil {
		return "", fmt.Errorf("write backup manifest: %w", err)
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("finalise backup archive: %w", err)
	}
	if err := archive.Sync(); err != nil {
		return "", fmt.Errorf("sync backup archive: %w", err)
	}
	if err := archive.Close(); err != nil {
		return "", fmt.Errorf("close backup archive: %w", err)
	}

	finalPath, err := uniqueBackupPath(dir, WorkspaceBackupExtension)
	if err != nil {
		return "", err
	}
	if err := os.Rename(tmpArchive, finalPath); err != nil {
		return "", fmt.Errorf("publish backup archive: %w", err)
	}
	committed = true
	return finalPath, nil
}

func ensurePrivateBackupDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

func addFileToArchive(zw *zip.Writer, sourcePath, archivePath string) (BackupManifestFile, error) {
	in, err := os.Open(sourcePath)
	if err != nil {
		return BackupManifestFile{}, err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return BackupManifestFile{}, err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return BackupManifestFile{}, err
	}
	header.Name = archivePath
	header.Method = zip.Deflate
	header.SetMode(0o640)
	out, err := zw.CreateHeader(header)
	if err != nil {
		return BackupManifestFile{}, err
	}
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, hash), in)
	if err != nil {
		return BackupManifestFile{}, err
	}
	return BackupManifestFile{Path: archivePath, Size: n, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func unusedTempPath(dir, pattern string) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	if err := os.Remove(name); err != nil {
		return "", err
	}
	return name, nil
}

func uniqueBackupPath(dir, extension string) (string, error) {
	stem := "backup-" + time.Now().Format("2006-01-02-150405")
	for i := 0; i < 1000; i++ {
		name := stem + extension
		if i > 0 {
			name = fmt.Sprintf("%s-%d%s", stem, i+1, extension)
		}
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", errors.New("could not allocate a unique backup filename")
}

// AutoBackup retains the original database-only automatic backup behaviour for
// callers that explicitly depend on it.
func (s *Store) AutoBackup(dir string, keep int) (string, error) {
	return s.autoBackup(dir, keep, ".db", func() (string, error) { return s.Backup(dir) })
}

// AutoBackupWorkspace creates at most one complete workspace backup per day.
func (s *Store) AutoBackupWorkspace(dataDir, dir, appVersion string, keep int) (string, error) {
	return s.autoBackup(dir, keep, WorkspaceBackupExtension, func() (string, error) {
		return s.BackupWorkspace(dataDir, dir, appVersion)
	})
}

func (s *Store) autoBackup(dir string, keep int, dailyExtension string, create func() (string, error)) (string, error) {
	today := time.Now().Format("2006-01-02")
	entries, _ := os.ReadDir(dir)
	var backups []string
	for _, e := range entries {
		if IsBackupName(e.Name()) {
			backups = append(backups, e.Name())
			if strings.HasPrefix(e.Name(), "backup-"+today) && strings.HasSuffix(e.Name(), dailyExtension) {
				return "", nil
			}
		}
	}
	path, err := create()
	if err != nil {
		return "", err
	}
	backups = append(backups, filepath.Base(path))
	sort.Strings(backups)
	for keep >= 0 && len(backups) > keep {
		if err := os.Remove(filepath.Join(dir, backups[0])); err != nil && !os.IsNotExist(err) {
			return path, fmt.Errorf("prune old backup: %w", err)
		}
		backups = backups[1:]
	}
	return path, nil
}

// BackupFile describes an existing backup on disk.
type BackupFile struct {
	Name        string
	Size        int64
	ModTime     time.Time
	IsWorkspace bool
}

// IsBackupName accepts current full-workspace archives and legacy .db files,
// while rejecting path traversal and unrelated files.
func IsBackupName(name string) bool {
	if filepath.Base(name) != name || strings.ContainsAny(name, `/\`) || !strings.HasPrefix(name, "backup-") {
		return false
	}
	return strings.HasSuffix(name, WorkspaceBackupExtension) || strings.HasSuffix(name, ".db")
}

// ListBackups returns backups in dir, newest first.
func ListBackups(dir string) []BackupFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []BackupFile
	for _, e := range entries {
		if !IsBackupName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, BackupFile{
			Name:        e.Name(),
			Size:        info.Size(),
			ModTime:     info.ModTime(),
			IsWorkspace: strings.HasSuffix(e.Name(), WorkspaceBackupExtension),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ModTime.Equal(out[j].ModTime) {
			return out[i].Name > out[j].Name
		}
		return out[i].ModTime.After(out[j].ModTime)
	})
	return out
}
