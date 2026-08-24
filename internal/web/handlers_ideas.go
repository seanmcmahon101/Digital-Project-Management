package web

import (
	"net/http"
)

func (s *Server) ideas(w http.ResponseWriter, r *http.Request) {
	list, err := s.St.Ideas()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "ideas", view{Title: "Portfolio & ideas", Active: "ideas",
		Data: map[string]any{"Ideas": list}})
}

func (s *Server) createIdea(w http.ResponseWriter, r *http.Request) {
	if _, err := s.St.CreateIdea(r.FormValue("title"), r.FormValue("summary"),
		r.FormValue("submitted_by")); err != nil {
		s.fail(w, r, "/ideas", err)
		return
	}
	s.redirect(w, r, "/ideas", "Idea added to the pipeline. Score it when you're ready to compare it against the others.")
}

func (s *Server) scoreIdea(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.ScoreIdea(id, formInt(r, "value"), formInt(r, "urgency"),
		formInt(r, "alignment"), formInt(r, "effort"), formInt(r, "risk")); err != nil {
		s.fail(w, r, "/ideas", err)
		return
	}
	s.redirect(w, r, "/ideas", "Scores saved.")
}

func (s *Server) ideaStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.SetIdeaStatus(id, r.FormValue("status")); err != nil {
		s.fail(w, r, "/ideas", err)
		return
	}
	s.redirect(w, r, "/ideas", "Idea updated.")
}

func (s *Server) convertIdea(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	p, err := s.St.ConvertIdea(id, r.FormValue("sponsor"), r.FormValue("lead"))
	if err != nil {
		s.fail(w, r, "/ideas", err)
		return
	}
	s.redirect(w, r, "/projects/"+itoa64(p.ID)+"/overview",
		p.Code+" created from the idea. The summary became the draft problem statement — refine it, then work through the Intake gate.")
}

func (s *Server) updateIdea(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.UpdateIdea(id, r.FormValue("title"), r.FormValue("summary"),
		r.FormValue("submitted_by")); err != nil {
		s.fail(w, r, "/ideas", err)
		return
	}
	s.redirect(w, r, "/ideas", "Idea updated.")
}

func (s *Server) deleteIdea(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w)
		return
	}
	if err := s.St.DeleteIdea(id); err != nil {
		s.fail(w, r, "/ideas", err)
		return
	}
	s.redirect(w, r, "/ideas", "Idea deleted.")
}
