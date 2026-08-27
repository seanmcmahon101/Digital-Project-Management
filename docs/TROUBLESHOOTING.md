# Troubleshooting

## The browser did not open

The app can still be running even if automatic browser launch fails.

1. Try `http://127.0.0.1:8383` in your browser.
2. Check `app.log` in the active data folder for a line beginning with `listening on`.
3. Open the exact address shown on that line.

On macOS or Linux, starting from a terminal also shows the address. Developers and advanced users can disable automatic launch with `--no-browser`.

## Port 8383 is already in use

Another copy of the application may already be running. Try the address above first. If a new instance selected a different free port, its `app.log` or terminal output shows the actual address.

Avoid running multiple copies against the same data folder. Close duplicate processes before continuing, particularly if you see database locking messages.

## I see an empty workspace

The application may have started with a different data folder.

1. Open **Settings → About** and note the displayed path.
2. Find the folder that contains your original `app.db` and `uploads/` directory.
3. Stop the application.
4. Restart it with `--data` pointing to that original folder.

Do not overwrite either folder until you have identified which contains the current data.

## Windows SmartScreen blocks the application

Release binaries are not currently code-signed. Confirm that the archive came from this repository's Releases page and compare its SHA-256 hash with `checksums.txt`. If the values match and you trust the source, use the option in the SmartScreen dialog to continue.

If your organisation manages application execution, ask its IT team to review or allow the binary rather than trying to bypass policy.

## macOS says the developer cannot be verified

Release binaries are not currently notarised. Verify the release checksum first. Then use **System Settings → Privacy & Security → Open Anyway** for this application. Do not disable Gatekeeper globally.

## Linux says “permission denied”

Restore the executable permission:

```sh
chmod +x digital-project-management
./digital-project-management
```

If the app cannot create its data folder, choose a location owned by your user account:

```sh
./digital-project-management --data "$HOME/.local/share/digital-project-management"
```

## The database is locked

Close other copies of the application that are using the same data directory, then start one copy again. If the problem remains:

1. Preserve the entire data folder before making changes.
2. Restart the computer to clear abandoned processes.
3. Review `app.log` for the first database-related error.

Do not delete `app.db-wal` or `app.db-shm` while any instance is running.

## A document is missing after restoring an older backup

Current `.dpm-backup` archives include uploaded documents and verify them before restore. Legacy `.db` backups preserve document records but do not contain uploaded file payloads. If you restored a legacy database on another computer, restore its matching `uploads/` directory from the same recovery copy. See [Data and backups](DATA_AND_BACKUPS.md).

## An import was rejected

Export the project register from **Settings** and use that file as the template. Keep the column headings unchanged. Project codes determine whether a row updates an existing project or creates a new one.

Imports are validated as a transaction: when validation fails, partial project updates should not be applied. Correct the error shown in the application and try again with a copy of the file.

## Collect information for a bug report

Include:

- Application version from **Settings → About**
- Operating system and processor type
- What you expected and what happened
- Exact steps that reproduce the problem
- Relevant lines from `app.log`

Remove project names, personal data, local usernames, and sensitive filesystem paths before attaching logs or screenshots. Submit the report using the [bug report form](https://github.com/seanmcmahon101/Digital-Project-Management/issues/new?template=bug.yml).
