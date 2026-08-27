# Digital Project Management

**Built to be very simple to use.**

<!--
Add the main product screenshot here. A 16:9 dashboard image works well.

Example:
![Digital Project Management dashboard](docs/images/dashboard.png)
-->

[![CI](https://github.com/seanmcmahon101/Digital-Project-Management/actions/workflows/ci.yml/badge.svg)](https://github.com/seanmcmahon101/Digital-Project-Management/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/seanmcmahon101/Digital-Project-Management)](https://github.com/seanmcmahon101/Digital-Project-Management/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A private, local-first operating system for taking digital projects from an initial idea through delivery, benefits tracking, and lessons learned.

Digital Project Management runs on your computer and opens in your normal web browser. It does not require an account, a cloud service, or a separate database server. Your project data remains in a local SQLite database that you control.

## Why I built it

I built Digital Project Management as my daily operating system for managing digital projects. It brings the working parts of delivery into one focused place: ideas, business cases, plans, RAID, RACI, decisions, scope, financials, benefits, and lessons learned.

The aim is not to add more process. It is to make good project governance easier to follow, keep the next important action visible, and maintain a clear record from the original problem through to the benefit delivered.

## What it includes

- Idea capture, scoring, prioritisation, and conversion into projects
- Project plans, tasks, milestones, delivery stages, and activity history
- RAID logs, decisions, stakeholders, and RACI responsibilities
- Scope baselines, requirements, tests, and change control
- Business cases, financial tracking, status snapshots, and PDF reports
- Benefits, implementation readiness, lessons learned, and closure support
- Document uploads and links, CSV/Excel transfer, and local backups
- Portfolio-wide dashboards and views for tasks, risks, decisions, and benefits
- Global workspace search, keyboard shortcuts, and responsive mobile navigation
- Project hold/resume controls, stage gates, coaching, and auditable decisions

## Download and run

No technical setup or Go installation is needed when you use a packaged release.

1. Open the [latest release](https://github.com/seanmcmahon101/Digital-Project-Management/releases/latest).
2. Download the archive that matches your computer.
3. Extract the archive, then run `digital-project-management` (`digital-project-management.exe` on Windows).

| Computer | Release archive |
| --- | --- |
| Windows, most PCs | `*_windows_amd64.zip` |
| Windows on ARM | `*_windows_arm64.zip` |
| macOS on Apple silicon | `*_darwin_arm64.tar.gz` |
| macOS on Intel | `*_darwin_amd64.tar.gz` |
| Linux, most PCs | `*_linux_amd64.tar.gz` |
| Linux on ARM64 | `*_linux_arm64.tar.gz` |

The app starts a private server on your computer and opens it in your default browser. Keep the application running while you use it. On Windows, leave the application console open; use `Ctrl+C` in that window when you are finished.

> [!NOTE]
> Release binaries are not currently code-signed. Windows SmartScreen or macOS Gatekeeper may therefore ask you to confirm that you trust the application. Only download releases from this repository, and verify the published checksum if you are unsure. See [Getting started](docs/GETTING_STARTED.md) for platform-specific guidance.

## Run from source

This path is intended for developers and contributors. Install the Go version declared in [`go.mod`](go.mod), then run:

```sh
git clone https://github.com/seanmcmahon101/Digital-Project-Management.git
cd Digital-Project-Management
go run .
```

The browser should open automatically. If it does not, check the terminal output or `app.log` for the local address.

## Your data

The active data folder is shown under **Settings → About**. It contains the database, log, backups, and any uploaded documents. New installations use your operating system's per-user configuration location. The application creates one database backup per day on startup and retains the latest 14.

Current `.dpm-backup` files are complete workspace archives: they contain a consistent database snapshot, uploaded documents, and checksums used during restore. Older `.db` backups contain database records only. Read [Data and backups](docs/DATA_AND_BACKUPS.md) before moving, restoring, or deleting application data.

## Documentation

- [Getting started](docs/GETTING_STARTED.md)
- [User guide](docs/USER_GUIDE.md)
- [Data and backups](docs/DATA_AND_BACKUPS.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [Development guide](docs/DEVELOPMENT.md)
- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)
- [Changelog](CHANGELOG.md)

## Privacy and intended use

The application listens on the loopback interface (`127.0.0.1`) and is designed for one person to use locally. It has no user accounts or network authentication. Do not expose its port to a network or the public internet.

## License

Digital Project Management is available under the [MIT License](LICENSE).
