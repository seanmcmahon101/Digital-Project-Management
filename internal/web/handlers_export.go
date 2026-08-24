package web

import (
	"bytes"
	"log"
	"net/http"

	"digipm/internal/coach"
	"digipm/internal/pdf"
)

// exportBusinessCase streams the project's business case as a PDF download.
func (s *Server) exportBusinessCase(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	snap, err := s.St.LoadSnapshot(id)
	if err != nil {
		s.notFound(w)
		return
	}
	var buf bytes.Buffer
	if err := pdf.BusinessCase(&buf, snap, s.currency(), s.St.Setting("org_name", "")); err != nil {
		log.Printf("business case pdf for project %d: %v", id, err)
		s.serverError(w, err)
		return
	}
	servePDF(w, buf.Bytes(), snap.Project.Code+"-business-case.pdf")
}

// exportStatusReport streams the project status report as a PDF download.
func (s *Server) exportStatusReport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	snap, err := s.St.LoadSnapshot(id)
	if err != nil {
		s.notFound(w)
		return
	}
	health := coach.Assess(snap)
	gate := coach.CheckGate(snap)
	var buf bytes.Buffer
	if err := pdf.StatusReport(&buf, snap, health, gate, s.currency(), s.St.Setting("org_name", "")); err != nil {
		log.Printf("status report pdf for project %d: %v", id, err)
		s.serverError(w, err)
		return
	}
	servePDF(w, buf.Bytes(), snap.Project.Code+"-status-report.pdf")
}

func servePDF(w http.ResponseWriter, data []byte, filename string) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Write(data)
}
