-- Core schema for the Digitalisation PM application.
-- Dates are stored as ISO strings: 'YYYY-MM-DD' for dates,
-- datetime('now') UTC timestamps for audit columns.

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE projects (
    id                INTEGER PRIMARY KEY,
    code              TEXT NOT NULL UNIQUE,           -- DPM-001
    name              TEXT NOT NULL,
    stage             TEXT NOT NULL DEFAULT 'intake'
                      CHECK (stage IN ('intake','discovery','define','plan','build','implement','benefits')),
    status            TEXT NOT NULL DEFAULT 'active'
                      CHECK (status IN ('active','on_hold','closed','cancelled')),
    sponsor           TEXT NOT NULL DEFAULT '',
    lead              TEXT NOT NULL DEFAULT '',
    department        TEXT NOT NULL DEFAULT '',
    problem_statement TEXT NOT NULL DEFAULT '',
    goal              TEXT NOT NULL DEFAULT '',
    current_state     TEXT NOT NULL DEFAULT '',       -- discovery: how the process works today
    business_case     TEXT NOT NULL DEFAULT '',
    scope_in          TEXT NOT NULL DEFAULT '',
    scope_out         TEXT NOT NULL DEFAULT '',
    start_date        TEXT NOT NULL DEFAULT '',
    target_end        TEXT NOT NULL DEFAULT '',
    go_live           TEXT NOT NULL DEFAULT '',
    closed_at         TEXT NOT NULL DEFAULT '',
    closure_summary   TEXT NOT NULL DEFAULT '',
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE ideas (
    id              INTEGER PRIMARY KEY,
    title           TEXT NOT NULL,
    summary         TEXT NOT NULL DEFAULT '',
    submitted_by    TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'new'
                    CHECK (status IN ('new','scored','approved','parked','rejected','converted')),
    score_value     INTEGER NOT NULL DEFAULT 0 CHECK (score_value BETWEEN 0 AND 5),
    score_urgency   INTEGER NOT NULL DEFAULT 0 CHECK (score_urgency BETWEEN 0 AND 5),
    score_alignment INTEGER NOT NULL DEFAULT 0 CHECK (score_alignment BETWEEN 0 AND 5),
    score_effort    INTEGER NOT NULL DEFAULT 0 CHECK (score_effort BETWEEN 0 AND 5),
    score_risk      INTEGER NOT NULL DEFAULT 0 CHECK (score_risk BETWEEN 0 AND 5),
    project_id      INTEGER REFERENCES projects(id) ON DELETE SET NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE gate_history (
    id              INTEGER PRIMARY KEY,
    project_id      INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    from_stage      TEXT NOT NULL,
    to_stage        TEXT NOT NULL,
    overridden      INTEGER NOT NULL DEFAULT 0,
    override_reason TEXT NOT NULL DEFAULT '',
    unmet_criteria  TEXT NOT NULL DEFAULT '',         -- newline-separated snapshot
    moved_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE milestones (
    id           INTEGER PRIMARY KEY,
    project_id   INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    due_date     TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE tasks (
    id           INTEGER PRIMARY KEY,
    project_id   INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    ref          TEXT NOT NULL,                        -- TASK-001
    title        TEXT NOT NULL,
    notes        TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'todo'
                 CHECK (status IN ('todo','doing','blocked','done')),
    priority     TEXT NOT NULL DEFAULT 'medium'
                 CHECK (priority IN ('low','medium','high')),
    assignee     TEXT NOT NULL DEFAULT '',
    due_date     TEXT NOT NULL DEFAULT '',
    milestone_id INTEGER REFERENCES milestones(id) ON DELETE SET NULL,
    completed_at TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, ref)
);

CREATE TABLE raid_items (
    id          INTEGER PRIMARY KEY,
    project_id  INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    ref         TEXT NOT NULL,                         -- RISK-001 / ISS-001 / ASM-001 / DEP-001
    kind        TEXT NOT NULL
                CHECK (kind IN ('risk','issue','assumption','dependency')),
    title       TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    owner       TEXT NOT NULL DEFAULT '',
    probability INTEGER NOT NULL DEFAULT 0 CHECK (probability BETWEEN 0 AND 5),
    impact      INTEGER NOT NULL DEFAULT 0 CHECK (impact BETWEEN 0 AND 5),
    mitigation  TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'open'
                CHECK (status IN ('open','closed')),
    due_date    TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, ref)
);

CREATE TABLE decisions (
    id         INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    ref        TEXT NOT NULL,                          -- DEC-001
    title      TEXT NOT NULL,
    context    TEXT NOT NULL DEFAULT '',
    outcome    TEXT NOT NULL DEFAULT '',
    decided_by TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending','decided')),
    decided_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, ref)
);

CREATE TABLE stakeholders (
    id         INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT '',
    influence  TEXT NOT NULL DEFAULT 'medium'
               CHECK (influence IN ('low','medium','high')),
    interest   TEXT NOT NULL DEFAULT 'medium'
               CHECK (interest IN ('low','medium','high')),
    attitude   TEXT NOT NULL DEFAULT 'neutral'
               CHECK (attitude IN ('champion','supportive','neutral','resistant')),
    notes      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE raci_activities (
    id         INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE raci_assignments (
    activity_id    INTEGER NOT NULL REFERENCES raci_activities(id) ON DELETE CASCADE,
    stakeholder_id INTEGER NOT NULL REFERENCES stakeholders(id) ON DELETE CASCADE,
    letter         TEXT NOT NULL CHECK (letter IN ('R','A','C','I')),
    PRIMARY KEY (activity_id, stakeholder_id)
);

CREATE TABLE requirements (
    id         INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    ref        TEXT NOT NULL,                          -- REQ-001
    title      TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT '',
    moscow     TEXT NOT NULL DEFAULT 'must'
               CHECK (moscow IN ('must','should','could','wont')),
    status     TEXT NOT NULL DEFAULT 'proposed'
               CHECK (status IN ('proposed','approved','delivered','dropped')),
    source     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, ref)
);

CREATE TABLE tests (
    id             INTEGER PRIMARY KEY,
    project_id     INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    ref            TEXT NOT NULL,                      -- TEST-001
    requirement_id INTEGER REFERENCES requirements(id) ON DELETE SET NULL,
    name           TEXT NOT NULL,
    steps          TEXT NOT NULL DEFAULT '',
    expected       TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'not_run'
                   CHECK (status IN ('not_run','pass','fail','blocked')),
    tested_by      TEXT NOT NULL DEFAULT '',
    tested_at      TEXT NOT NULL DEFAULT '',
    notes          TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, ref)
);

CREATE TABLE change_requests (
    id          INTEGER PRIMARY KEY,
    project_id  INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    ref         TEXT NOT NULL,                         -- CR-001
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    impact      TEXT NOT NULL DEFAULT '',
    raised_by   TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'proposed'
                CHECK (status IN ('proposed','approved','rejected','withdrawn')),
    decided_at  TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, ref)
);

CREATE TABLE pain_points (
    id           INTEGER PRIMARY KEY,
    project_id   INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    description  TEXT NOT NULL,
    process_area TEXT NOT NULL DEFAULT '',
    impact       TEXT NOT NULL DEFAULT 'medium'
                 CHECK (impact IN ('low','medium','high')),
    frequency    TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE benefits (
    id             INTEGER PRIMARY KEY,
    project_id     INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    ref            TEXT NOT NULL,                      -- BEN-001
    name           TEXT NOT NULL,
    category       TEXT NOT NULL DEFAULT 'custom'
                   CHECK (category IN ('hours_saved','cost_saved','error_reduction','cycle_time',
                                       'quality','compliance','reporting_speed','adoption','custom')),
    unit           TEXT NOT NULL DEFAULT '',           -- e.g. 'hrs/month', '%', 'days'
    direction      TEXT NOT NULL DEFAULT 'decrease'
                   CHECK (direction IN ('increase','decrease')),
    baseline_value REAL,                               -- NULL = not yet measured
    baseline_date  TEXT NOT NULL DEFAULT '',
    target_value   REAL,                               -- NULL = not yet set
    annual_value   REAL NOT NULL DEFAULT 0,            -- expected GBP per year if target achieved
    notes          TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at     TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, ref)
);

CREATE TABLE benefit_measurements (
    id          INTEGER PRIMARY KEY,
    benefit_id  INTEGER NOT NULL REFERENCES benefits(id) ON DELETE CASCADE,
    value       REAL NOT NULL,
    measured_at TEXT NOT NULL DEFAULT '',
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE readiness_items (
    id         INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    category   TEXT NOT NULL DEFAULT 'technical'
               CHECK (category IN ('training','communications','support','data','technical','rollback')),
    item       TEXT NOT NULL,
    owner      TEXT NOT NULL DEFAULT '',
    done       INTEGER NOT NULL DEFAULT 0,
    notes      TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE lessons (
    id             INTEGER PRIMARY KEY,
    project_id     INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    category       TEXT NOT NULL DEFAULT 'went_well'
                   CHECK (category IN ('went_well','went_wrong','do_differently')),
    lesson         TEXT NOT NULL,
    recommendation TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE activity_log (
    id         INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    entity     TEXT NOT NULL DEFAULT '',
    entity_ref TEXT NOT NULL DEFAULT '',
    action     TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE ref_counters (
    project_id INTEGER NOT NULL,                       -- 0 = global (project codes)
    kind       TEXT NOT NULL,
    next       INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (project_id, kind)
);

CREATE INDEX idx_tasks_project      ON tasks(project_id);
CREATE INDEX idx_raid_project       ON raid_items(project_id);
CREATE INDEX idx_decisions_project  ON decisions(project_id);
CREATE INDEX idx_requirements_proj  ON requirements(project_id);
CREATE INDEX idx_tests_project      ON tests(project_id);
CREATE INDEX idx_benefits_project   ON benefits(project_id);
CREATE INDEX idx_measure_benefit    ON benefit_measurements(benefit_id);
CREATE INDEX idx_activity_project   ON activity_log(project_id, id);
CREATE INDEX idx_milestones_project ON milestones(project_id);
