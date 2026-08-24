package web

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const maxDocumentSize = 50 << 20 // 50 MiB

func (s *Server) uploadDocument(w http.ResponseWriter, r *http.Request) {
	projectID, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := "/projects/" + itoa64(projectID) + "/documents"
	if _, err := s.St.Project(projectID); err != nil {
		s.notFound(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxDocumentSize+(1<<20))
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		s.fail(w, r, back, errValidation("Choose a file up to 50 MB"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	in, header, err := r.FormFile("file")
	if err != nil {
		s.fail(w, r, back, errValidation("Choose a file to upload"))
		return
	}
	defer in.Close()

	original := filepath.Base(strings.TrimSpace(header.Filename))
	if original == "" || original == "." {
		s.fail(w, r, back, errValidation("The uploaded file needs a filename"))
		return
	}
	stored, err := opaqueFilename(original)
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	dir := filepath.Join(s.DataDir, "uploads", itoa64(projectID))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		s.fail(w, r, back, err)
		return
	}
	path, ok := safeUploadPath(dir, stored)
	if !ok {
		s.fail(w, r, back, errors.New("unsafe generated document path"))
		return
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		s.fail(w, r, back, err)
		return
	}
	defer func() { out.Close() }()

	var head [512]byte
	n, readErr := io.ReadFull(in, head[:])
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		_ = out.Close()
		os.Remove(path)
		s.fail(w, r, back, readErr)
		return
	}
	if _, err = out.Write(head[:n]); err == nil {
		var copied int64
		copied, err = io.Copy(out, io.LimitReader(in, maxDocumentSize-int64(n)+1))
		n += int(copied)
		if int64(n) > maxDocumentSize {
			err = errValidation("Files must be 50 MB or smaller")
		}
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path)
		s.fail(w, r, back, err)
		return
	}
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(head[:n])
	}
	if err := s.St.AddFileDocument(projectID, r.FormValue("title"), r.FormValue("description"),
		original, stored, mimeType, int64(n)); err != nil {
		os.Remove(path)
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Document uploaded.")
}

func (s *Server) addDocumentLink(w http.ResponseWriter, r *http.Request) {
	projectID, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	back := "/projects/" + itoa64(projectID) + "/documents"
	if err := s.St.AddLinkDocument(projectID, r.FormValue("title"), r.FormValue("description"), r.FormValue("url")); err != nil {
		s.fail(w, r, back, err)
		return
	}
	s.redirect(w, r, back, "Document link added.")
}

func (s *Server) downloadDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	d, err := s.St.Document(id)
	if err != nil || d.Kind != "file" {
		s.notFound(w)
		return
	}
	dir := filepath.Join(s.DataDir, "uploads", itoa64(d.ProjectID))
	path, ok := safeUploadPath(dir, d.StoredName)
	if !ok {
		s.notFound(w)
		return
	}
	if _, err := os.Stat(path); err != nil {
		s.notFound(w)
		return
	}
	filename := strings.ReplaceAll(strings.ReplaceAll(d.OriginalName, "\r", ""), "\n", "")
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Type", d.MimeType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

func (s *Server) deleteDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	d, err := s.St.Document(id)
	if err != nil {
		s.notFound(w)
		return
	}
	back := "/projects/" + itoa64(d.ProjectID) + "/documents"
	if err := s.St.DeleteDocument(id); err != nil {
		s.fail(w, r, back, err)
		return
	}
	if d.Kind == "file" {
		dir := filepath.Join(s.DataDir, "uploads", itoa64(d.ProjectID))
		if path, safe := safeUploadPath(dir, d.StoredName); safe {
			_ = os.Remove(path)
		}
	}
	s.redirect(w, r, back, "Document removed.")
}

func opaqueFilename(original string) (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("generate document filename: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(original))
	if len(ext) > 12 {
		ext = ""
	}
	for _, r := range strings.TrimPrefix(ext, ".") {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			ext = ""
			break
		}
	}
	return hex.EncodeToString(token[:]) + ext, nil
}

// safeUploadPath enforces that an opaque database filename stays directly
// beneath its assigned project directory, even if the database is tampered.
func safeUploadPath(dir, stored string) (string, bool) {
	if stored == "" || filepath.Base(stored) != stored || strings.ContainsAny(stored, `/\`) {
		return "", false
	}
	base, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	candidate, err := filepath.Abs(filepath.Join(base, stored))
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(base, candidate)
	return candidate, err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
