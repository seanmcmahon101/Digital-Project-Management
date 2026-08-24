package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Backup writes a consistent copy of the database into dir using
// VACUUM INTO and returns the created file path.
func (s *Store) Backup(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	name := fmt.Sprintf("backup-%s.db", time.Now().Format("2006-01-02-150405"))
	path := filepath.Join(dir, name)
	// VACUUM INTO refuses to overwrite; the timestamped name makes
	// collisions practically impossible.
	if _, err := s.DB.Exec(`VACUUM INTO ?`, path); err != nil {
		return "", fmt.Errorf("backup database: %w", err)
	}
	return path, nil
}

// AutoBackup creates at most one backup per day in dir and prunes old
// automatic backups beyond keep files. Returns the path if one was made.
func (s *Store) AutoBackup(dir string, keep int) (string, error) {
	today := time.Now().Format("2006-01-02")
	entries, _ := os.ReadDir(dir)
	var backups []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "backup-") && strings.HasSuffix(e.Name(), ".db") {
			backups = append(backups, e.Name())
			if strings.HasPrefix(e.Name(), "backup-"+today) {
				return "", nil // already backed up today
			}
		}
	}
	path, err := s.Backup(dir)
	if err != nil {
		return "", err
	}
	// Prune oldest beyond keep (names sort chronologically).
	backups = append(backups, filepath.Base(path))
	sort.Strings(backups)
	for len(backups) > keep {
		os.Remove(filepath.Join(dir, backups[0]))
		backups = backups[1:]
	}
	return path, nil
}

// BackupFile describes an existing backup on disk.
type BackupFile struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// ListBackups returns backups in dir, newest first.
func ListBackups(dir string) []BackupFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []BackupFile
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "backup-") || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupFile{Name: e.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out
}
