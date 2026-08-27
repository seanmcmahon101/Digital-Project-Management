# Contributing

Thank you for helping improve Digital Project Management. Contributions should preserve the product's local-first approach and keep common project-management tasks understandable without specialist training.

## Before starting

- Search existing issues and pull requests to avoid duplicate work.
- Open a feature request before a large behavioral, schema, or interface change.
- Do not include real project records, personal data, credentials, or proprietary documents in issues, tests, or commits.
- Use GitHub's private vulnerability-reporting route for security concerns; see [SECURITY.md](SECURITY.md).

## Development setup

Follow the [development guide](docs/DEVELOPMENT.md) to install prerequisites and run the application from source.

Create a focused branch from the current `main` branch:

```sh
git switch main
git pull --ff-only
git switch -c change/short-description
```

## Expectations

- Keep changes focused and explain the user problem they solve.
- Prefer plain language and progressive disclosure in the interface.
- Add or update tests for behavior changes and regressions.
- Add a new migration rather than modifying an existing released migration.
- Update user documentation when behavior, setup, storage, or recovery changes.
- Preserve compatibility with Windows, macOS, and Linux unless a limitation is documented and agreed.
- Avoid adding dependencies when the standard library or existing stack is sufficient.

## Validate the change

Run:

```sh
gofmt -w .
go mod tidy
go vet ./...
go test -race -count=1 ./...
go build -trimpath .
```

Also exercise the changed workflow in a browser. For interface changes, check keyboard operation, visible focus, narrow screens, readable contrast, empty states, validation errors, and confirmation of destructive actions.

## Pull requests

A good pull request includes:

- A concise explanation of the problem and solution
- The affected user workflows
- Testing performed
- Screenshots for meaningful visual changes
- Migration, compatibility, privacy, or recovery implications
- Documentation and changelog updates where relevant

Keep unrelated formatting and refactoring out of the same pull request. Maintainers may ask for a change to be split when that makes review or rollback safer.

By contributing, you agree that your contribution is licensed under the repository's [MIT License](LICENSE).
