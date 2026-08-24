-- Project evidence: locally stored attachments and safe external links.
CREATE TABLE project_documents (
    id            INTEGER PRIMARY KEY,
    project_id    INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL CHECK (kind IN ('file','link')),
    title         TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    original_name TEXT NOT NULL DEFAULT '',
    stored_name   TEXT NOT NULL DEFAULT '',
    url           TEXT NOT NULL DEFAULT '',
    mime_type     TEXT NOT NULL DEFAULT '',
    size_bytes    INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_project_documents_project ON project_documents(project_id, id DESC);
