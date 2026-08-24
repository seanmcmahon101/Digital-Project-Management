package store

import (
	"database/sql"
	"errors"
	"net/url"
	"strings"
)

// ProjectDocument is either an uploaded local file or an external web link.
// StoredName is an opaque server-generated filename and is never user input.
type ProjectDocument struct {
	ID           int64
	ProjectID    int64
	Kind         string
	Title        string
	Description  string
	OriginalName string
	StoredName   string
	URL          string
	MimeType     string
	SizeBytes    int64
	CreatedAt    string
}

func (s *Store) Documents(projectID int64) ([]ProjectDocument, error) {
	rows, err := s.DB.Query(`SELECT id, project_id, kind, title, description,
		original_name, stored_name, url, mime_type, size_bytes, created_at
		FROM project_documents WHERE project_id = ? ORDER BY id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectDocument
	for rows.Next() {
		var d ProjectDocument
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Kind, &d.Title, &d.Description,
			&d.OriginalName, &d.StoredName, &d.URL, &d.MimeType, &d.SizeBytes, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) Document(id int64) (ProjectDocument, error) {
	var d ProjectDocument
	err := s.DB.QueryRow(`SELECT id, project_id, kind, title, description,
		original_name, stored_name, url, mime_type, size_bytes, created_at
		FROM project_documents WHERE id = ?`, id).Scan(&d.ID, &d.ProjectID, &d.Kind,
		&d.Title, &d.Description, &d.OriginalName, &d.StoredName, &d.URL,
		&d.MimeType, &d.SizeBytes, &d.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

func (s *Store) AddFileDocument(projectID int64, title, description, originalName, storedName, mimeType string, size int64) error {
	title = strings.TrimSpace(title)
	if title == "" {
		title = originalName
	}
	if err := Validate(Require(title, "Document title"), Require(originalName, "Filename"), Require(storedName, "Stored filename")); err != nil {
		return err
	}
	_, err := s.DB.Exec(`INSERT INTO project_documents
		(project_id, kind, title, description, original_name, stored_name, mime_type, size_bytes)
		VALUES (?, 'file', ?, ?, ?, ?, ?, ?)`, projectID, title, strings.TrimSpace(description),
		originalName, storedName, mimeType, size)
	if err == nil {
		s.LogActivity(projectID, "document", "", "uploaded", title)
	}
	return err
}

func (s *Store) AddLinkDocument(projectID int64, title, description, rawURL string) error {
	title, rawURL = strings.TrimSpace(title), strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return &ValidationError{Problems: []string{"Document link must be a complete http:// or https:// address"}}
	}
	if err := Validate(Require(title, "Document title")); err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT INTO project_documents
		(project_id, kind, title, description, url) VALUES (?, 'link', ?, ?, ?)`,
		projectID, title, strings.TrimSpace(description), u.String())
	if err == nil {
		s.LogActivity(projectID, "document", "", "linked", title)
	}
	return err
}

func (s *Store) DeleteDocument(id int64) error {
	d, err := s.Document(id)
	if err != nil {
		return err
	}
	if _, err := s.DB.Exec(`DELETE FROM project_documents WHERE id = ?`, id); err != nil {
		return err
	}
	s.LogActivity(d.ProjectID, "document", "", "deleted", d.Title)
	return nil
}
