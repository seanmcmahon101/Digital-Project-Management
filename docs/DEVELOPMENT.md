# Development guide

## Prerequisites

- Git
- The Go version declared by the `go` directive in [`go.mod`](../go.mod)
- A current browser

The application uses a pure-Go SQLite driver, so a C compiler or separately installed database server is not required.

## Set up a development copy

```sh
git clone https://github.com/seanmcmahon101/Digital-Project-Management.git
cd Digital-Project-Management
go mod download
go run . --data ./data
```

The app listens on loopback and normally opens a browser automatically. Use a disposable data directory when working on migrations or destructive behavior.

Useful runtime flags:

```text
--port 8383       local port; 0 asks the operating system for a free port
--no-browser      do not open the browser automatically
--data PATH       use an explicit application data directory
--portable        keep data in a directory beside the executable
--version         print build version information and exit
```

Run the compiled binary with `--help` to see the authoritative list for that version.

## Common checks

Run these before submitting a pull request:

```sh
gofmt -w .
go mod tidy
go vet ./...
go test -race -count=1 ./...
go build -trimpath .
```

CI repeats formatting, module, vet, test, and build checks. Tests also run natively on Linux, Windows, and macOS.

## Repository layout

| Path | Responsibility |
| --- | --- |
| `main.go` | Process lifecycle, command-line options, local listener, and browser launch |
| `internal/db/` | Database connection and embedded schema migrations |
| `internal/store/` | Data access, validation, backups, and transfer operations |
| `internal/coach/` | Project health and guidance rules |
| `internal/pdf/` | PDF report generation |
| `internal/web/` | HTTP handlers, templates, and embedded browser assets |
| `docs/` | User and contributor documentation |
| `.github/workflows/` | CI and release automation |

The browser assets and templates are embedded in the executable, so a release is a self-contained binary.

## Database migrations

Migrations live in `internal/db/migrations/` and run in filename order. For a schema change:

1. Add a new, monotonically numbered SQL file; never rewrite a migration that may already have run for a user.
2. Make the migration safe for databases created by earlier released versions.
3. Add tests covering migration and store behavior.
4. Verify the application against a copied real-world database, never the only copy.

Current `.dpm-backup` archives contain the database and uploaded document payloads. Legacy `.db` files contain only database records; retain compatibility with both formats when changing restore behavior.

## Release build metadata

The release workflow expects three package-level string variables in `main`:

- `main.version`
- `main.commit`
- `main.buildDate`

They are populated with linker `-X` flags. Development builds should provide useful fallback values such as `dev`, `unknown`, and an empty date. Keep this contract aligned with [`.github/workflows/release.yml`](../.github/workflows/release.yml).

## Release process

Releases are created from semantic version tags on `main`:

```sh
git switch main
git pull --ff-only
go test ./...
git tag -a v1.2.3 -m "Digital Project Management v1.2.3"
git push origin v1.2.3
```

The release workflow validates the tag, runs tests, cross-compiles six platform archives, generates SHA-256 checksums, and publishes a GitHub release with generated notes. Do not upload locally built replacements to an existing tag. Correct the problem and publish a new version instead.

Release archive and executable names use `digital-project-management`. The existing user-interface name may remain `DigitalisationPM` for compatibility until a deliberate migration is completed.
