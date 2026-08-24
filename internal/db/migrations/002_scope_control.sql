-- Structured scope, immutable scope approvals, and traceable change impact.

CREATE TABLE scope_items (
    id                  INTEGER PRIMARY KEY,
    project_id          INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    ref                 TEXT NOT NULL,
    classification      TEXT NOT NULL DEFAULT 'in'
                        CHECK (classification IN ('in','out')),
    title               TEXT NOT NULL,
    owner               TEXT NOT NULL DEFAULT '',
    rationale           TEXT NOT NULL DEFAULT '',
    acceptance_criteria TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'proposed'
                        CHECK (status IN ('proposed','agreed','delivered','removed')),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, ref)
);

CREATE TABLE scope_baselines (
    id             INTEGER PRIMARY KEY,
    project_id     INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version        INTEGER NOT NULL,
    approved_by    TEXT NOT NULL,
    approved_at    TEXT NOT NULL,
    notes          TEXT NOT NULL DEFAULT '',
    scope_snapshot TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, version)
);

ALTER TABLE change_requests ADD COLUMN cost_impact REAL NOT NULL DEFAULT 0;
ALTER TABLE change_requests ADD COLUMN schedule_impact_days INTEGER NOT NULL DEFAULT 0;
ALTER TABLE change_requests ADD COLUMN target_date_impact TEXT NOT NULL DEFAULT '';

CREATE TABLE change_scope_items (
    change_request_id INTEGER NOT NULL REFERENCES change_requests(id) ON DELETE CASCADE,
    scope_item_id     INTEGER NOT NULL REFERENCES scope_items(id) ON DELETE CASCADE,
    PRIMARY KEY (change_request_id, scope_item_id)
);

CREATE TABLE change_requirements (
    change_request_id INTEGER NOT NULL REFERENCES change_requests(id) ON DELETE CASCADE,
    requirement_id    INTEGER NOT NULL REFERENCES requirements(id) ON DELETE CASCADE,
    PRIMARY KEY (change_request_id, requirement_id)
);

CREATE INDEX idx_scope_items_project ON scope_items(project_id, classification, ref);
CREATE INDEX idx_scope_baselines_project ON scope_baselines(project_id, version DESC);
CREATE INDEX idx_change_scope_item ON change_scope_items(scope_item_id);
CREATE INDEX idx_change_requirement ON change_requirements(requirement_id);
