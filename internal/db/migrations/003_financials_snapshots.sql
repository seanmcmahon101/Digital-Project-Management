-- Project-level investment tracking and immutable manual status baselines.

CREATE TABLE project_financials (
    project_id       INTEGER PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    estimated_cost   REAL NOT NULL DEFAULT 0 CHECK (estimated_cost >= 0),
    approved_budget  REAL NOT NULL DEFAULT 0 CHECK (approved_budget >= 0),
    actual_cost      REAL NOT NULL DEFAULT 0 CHECK (actual_cost >= 0),
    notes            TEXT NOT NULL DEFAULT '',
    updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE project_status_snapshots (
    id                    INTEGER PRIMARY KEY,
    project_id            INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    stage                 TEXT NOT NULL,
    project_status        TEXT NOT NULL,
    health_status         TEXT NOT NULL CHECK (health_status IN ('green','amber','red','closed')),
    health_summary        TEXT NOT NULL DEFAULT '',
    total_tasks           INTEGER NOT NULL DEFAULT 0 CHECK (total_tasks >= 0),
    done_tasks            INTEGER NOT NULL DEFAULT 0 CHECK (done_tasks >= 0),
    open_raid_items       INTEGER NOT NULL DEFAULT 0 CHECK (open_raid_items >= 0),
    overdue_tasks         INTEGER NOT NULL DEFAULT 0 CHECK (overdue_tasks >= 0),
    overdue_milestones    INTEGER NOT NULL DEFAULT 0 CHECK (overdue_milestones >= 0),
    expected_annual_value REAL NOT NULL DEFAULT 0,
    realised_annual_value REAL NOT NULL DEFAULT 0,
    estimated_cost        REAL NOT NULL DEFAULT 0,
    approved_budget       REAL NOT NULL DEFAULT 0,
    actual_cost           REAL NOT NULL DEFAULT 0,
    note                  TEXT NOT NULL DEFAULT '',
    captured_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_status_snapshots_project
    ON project_status_snapshots(project_id, id DESC);
