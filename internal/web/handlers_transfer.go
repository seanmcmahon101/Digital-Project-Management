package web

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"digipm/internal/store"
)

const maxTransferSize = 20 << 20

func (s *Server) exportProjectsCSV(w http.ResponseWriter, r *http.Request) {
	projects, err := s.St.Projects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="projects-export.csv"`)
	if err := writeProjectsCSV(w, store.ProjectTransferRows(projects)); err != nil {
		return
	}
}

func (s *Server) exportProjectsXLSX(w http.ResponseWriter, r *http.Request) {
	projects, err := s.St.Projects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	var workbook bytes.Buffer
	if err := writeProjectsXLSX(&workbook, store.ProjectTransferRows(projects)); err != nil {
		s.serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="projects-export.xlsx"`)
	w.Header().Set("Content-Length", fmt.Sprint(workbook.Len()))
	_, _ = workbook.WriteTo(w)
}

func (s *Server) importProjects(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxTransferSize+(1<<20))
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		s.fail(w, r, "/settings", errValidation("Choose a CSV or XLSX file up to 20 MB"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		s.fail(w, r, "/settings", errValidation("Choose a CSV or XLSX file to import"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxTransferSize+1))
	if err != nil || len(data) > maxTransferSize {
		s.fail(w, r, "/settings", errValidation("Import files must be 20 MB or smaller"))
		return
	}
	ext := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	var rows []store.ProjectTransferRow
	switch ext {
	case ".csv":
		rows, err = readProjectsCSV(bytes.NewReader(data))
	case ".xlsx":
		rows, err = readProjectsXLSX(data)
	default:
		err = fmt.Errorf("unsupported file type %q", ext)
	}
	if err != nil {
		s.fail(w, r, "/settings", errValidation("Could not import the spreadsheet: "+err.Error()))
		return
	}
	created, updated, err := s.St.ImportProjectRows(rows)
	if err != nil {
		s.fail(w, r, "/settings", err)
		return
	}
	s.redirect(w, r, "/settings", fmt.Sprintf("Import complete: %d project(s) created, %d updated.", created, updated))
}
