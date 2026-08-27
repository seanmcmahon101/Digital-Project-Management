package store

import (
	"fmt"
	"strings"
)

// SearchResult is a lightweight cross-project match used by global search.
// Link points to the workspace tab where the record can be viewed or edited.
type SearchResult struct {
	Kind        string
	KindLabel   string
	EntityID    int64
	ProjectID   int64
	Ref         string
	Title       string
	Summary     string
	ProjectCode string
	ProjectName string
	Link        string
}

// Search finds records across the portfolio. It deliberately uses ordinary
// LIKE queries rather than a second search index: local datasets are small and
// keeping one source of truth makes backups and migrations more dependable.
func (s *Store) Search(query string, limit int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	needle := "%" + escapeLike(strings.ToLower(query)) + "%"
	rows, err := s.DB.Query(`
		SELECT kind, entity_id, project_id, ref, title, summary, project_code, project_name
		FROM (
			SELECT 'project' AS kind, p.id AS entity_id, p.id AS project_id,
				p.code AS ref, p.name AS title,
				trim(p.problem_statement || ' ' || p.goal || ' ' || p.department) AS summary,
				p.code AS project_code, p.name AS project_name
			FROM projects p
			UNION ALL
			SELECT 'idea', i.id, COALESCE(i.project_id, 0), '', i.title,
				trim(i.summary || ' ' || i.submitted_by),
				COALESCE(p.code, ''), COALESCE(p.name, '')
			FROM ideas i LEFT JOIN projects p ON p.id = i.project_id
			UNION ALL
			SELECT 'task', t.id, t.project_id, t.ref, t.title,
				trim(t.notes || ' ' || t.assignee), p.code, p.name
			FROM tasks t JOIN projects p ON p.id = t.project_id
			UNION ALL
			SELECT 'raid', r.id, r.project_id, r.ref, r.title,
				trim(r.detail || ' ' || r.mitigation || ' ' || r.owner), p.code, p.name
			FROM raid_items r JOIN projects p ON p.id = r.project_id
			UNION ALL
			SELECT 'decision', d.id, d.project_id, d.ref, d.title,
				trim(d.context || ' ' || d.outcome || ' ' || d.decided_by), p.code, p.name
			FROM decisions d JOIN projects p ON p.id = d.project_id
			UNION ALL
			SELECT 'requirement', req.id, req.project_id, req.ref, req.title,
				trim(req.detail || ' ' || req.source), p.code, p.name
			FROM requirements req JOIN projects p ON p.id = req.project_id
			UNION ALL
			SELECT 'change', c.id, c.project_id, c.ref, c.title,
				trim(c.description || ' ' || c.impact || ' ' || c.raised_by), p.code, p.name
			FROM change_requests c JOIN projects p ON p.id = c.project_id
			UNION ALL
			SELECT 'benefit', b.id, b.project_id, b.ref, b.name,
				trim(b.notes || ' ' || b.category || ' ' || b.unit), p.code, p.name
			FROM benefits b JOIN projects p ON p.id = b.project_id
			UNION ALL
			SELECT 'document', doc.id, doc.project_id, '', doc.title,
				trim(doc.description || ' ' || doc.original_name || ' ' || doc.url), p.code, p.name
			FROM project_documents doc JOIN projects p ON p.id = doc.project_id
			UNION ALL
			SELECT 'stakeholder', sh.id, sh.project_id, '', sh.name,
				trim(sh.role || ' ' || sh.notes), p.code, p.name
			FROM stakeholders sh JOIN projects p ON p.id = sh.project_id
			UNION ALL
			SELECT 'lesson', l.id, l.project_id, '', l.lesson,
				l.recommendation, p.code, p.name
			FROM lessons l JOIN projects p ON p.id = l.project_id
		) searchable
		WHERE lower(ref || ' ' || title || ' ' || summary || ' ' || project_code || ' ' || project_name)
			LIKE ? ESCAPE '\'
		ORDER BY CASE kind WHEN 'project' THEN 0 WHEN 'idea' THEN 1 ELSE 2 END,
			project_code, title
		LIMIT ?`, needle, limit)
	if err != nil {
		return nil, fmt.Errorf("search portfolio: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.Kind, &result.EntityID, &result.ProjectID, &result.Ref,
			&result.Title, &result.Summary, &result.ProjectCode, &result.ProjectName); err != nil {
			return nil, err
		}
		result.KindLabel, result.Link = searchResultDestination(result)
		results = append(results, result)
	}
	return results, rows.Err()
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}

func searchResultDestination(result SearchResult) (string, string) {
	projectLink := func(tab string) string {
		return fmt.Sprintf("/projects/%d/%s", result.ProjectID, tab)
	}
	switch result.Kind {
	case "project":
		return "Project", projectLink("overview")
	case "idea":
		return "Idea", fmt.Sprintf("/ideas#idea-%d", result.EntityID)
	case "task":
		return "Task", projectLink("board")
	case "raid":
		return "RAID", projectLink("raid")
	case "decision":
		return "Decision", projectLink("decisions")
	case "requirement":
		return "Requirement", projectLink("requirements")
	case "change":
		return "Change request", projectLink("changes")
	case "benefit":
		return "Benefit", projectLink("benefits")
	case "document":
		return "Document", projectLink("documents")
	case "stakeholder":
		return "Stakeholder", projectLink("people")
	case "lesson":
		return "Lesson", projectLink("close")
	default:
		return "Result", "/"
	}
}
