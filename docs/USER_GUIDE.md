# User guide

Digital Project Management supports a project from early discovery through delivery and closure. You can use only the parts that help your process; most sections work independently.

## Start with an idea or a project

Use **Ideas** when a proposal still needs comparison or approval. Record the problem, expected outcome, sponsor, effort, value, urgency, and risk. Scoring provides a consistent way to compare proposals. When an idea is ready, convert it into a project so its original context is retained.

Use **New project** when work has already been approved. Give each project a short, unique code; that code is also used to match rows during spreadsheet imports.

## Build the project foundation

The project workspace groups information by purpose:

- **Overview** summarises status, ownership, health, and next actions.
- **Discovery** records pain points and evidence behind the work.
- **Business case** captures rationale, scope, costs, benefits, and approvals.
- **People** stores stakeholders and RACI responsibilities.
- **Requirements** links delivery requirements with validation and tests.
- **Plan** contains tasks and milestones.
- **RAID** tracks risks, assumptions, issues, and dependencies.
- **Decisions** preserves significant choices and their rationale.
- **Changes** records proposed scope or delivery changes and their outcome.
- **Implementation** tracks readiness for adoption and go-live.
- **Benefits** records expected outcomes and later measurements.
- **Documents** stores local files or links to material held elsewhere.
- **Close** supports closure information and lessons learned.

Use the project health and guidance panels as prompts, not as mandatory gates. They highlight missing or overdue information so that you can decide what matters for the project.

## Plan and control delivery

Add clear task owners and due dates, then use the portfolio-wide **Tasks** view to see work across projects. Milestones are best reserved for important delivery or approval dates.

Review the RAID log regularly. Keep each item owned, give it a realistic impact and likelihood where applicable, and close it when it no longer requires attention.

Record decisions when they affect scope, cost, timing, architecture, or stakeholders. A short rationale is valuable later, particularly when a project changes hands.

For controlled scope:

1. Add structured in-scope and out-of-scope items.
2. Approve a scope baseline when the boundary is agreed.
3. Use change requests for later movement against that baseline.

Each baseline is versioned and keeps a snapshot of the scope at the time of approval.

## Track status and finances

Keep overall project status and commentary current on the project overview. Capture status snapshots at reporting points to preserve the history rather than continually replacing the previous position.

Financial tracking supports planned and actual values. Use consistent currency and rate settings across the portfolio so totals remain meaningful.

## Reports and data transfer

Use **Reports** for portfolio reporting and project PDF outputs. Review a generated report before sharing it, especially when optional project fields have not been completed.

Under **Settings**, you can export the project register as CSV or Excel. Start from an export before preparing an import because it provides the expected column names and formats. Imports update projects with matching project codes and create projects with new codes; they are validated and applied as one transaction.

## Documents

Each project can contain:

- Uploaded files of up to 50 MB, stored inside the local application data folder
- HTTPS or HTTP links to documents stored in an existing document-management system

Removing a file document from the project also removes its stored local file. Current complete workspace backups include uploaded files; older database-only `.db` backups do not. See [Data and backups](DATA_AND_BACKUPS.md).

## A useful review rhythm

A lightweight operating rhythm keeps the workspace useful:

- Weekly: tasks, milestones, RAID items, decisions, and project status
- At approvals: business case, scope baseline, readiness, and status snapshot
- Monthly or at reporting periods: finances, benefits, and portfolio reports
- At closure: outstanding actions, final benefit ownership, documents, and lessons

The best record is a concise one that is kept current. Avoid filling fields solely for completeness.
