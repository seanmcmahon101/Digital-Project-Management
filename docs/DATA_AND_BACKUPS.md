# Data and backups

Digital Project Management keeps its working data on your computer. Understanding the data folder is important before upgrading, moving machines, or restoring a backup.

## Find the active data folder

Open **Settings → About**. The displayed path is the data folder used by the running application.

New installations use the following per-user configuration locations:

| Platform | Default location |
| --- | --- |
| Windows | `%AppData%\DigitalisationPM\data` |
| macOS | `~/Library/Application Support/DigitalisationPM/data` |
| Linux | `$XDG_CONFIG_HOME/DigitalisationPM/data`, normally `~/.config/DigitalisationPM/data` |

To avoid losing sight of an existing workspace after an upgrade, the application continues to use a legacy `data` folder beside the executable when that folder already contains `app.db`.

An explicit location can be selected when starting the app:

```sh
digital-project-management --data "/path/to/project-data"
```

On Windows PowerShell:

```powershell
.\digital-project-management.exe --data "D:\Digital Project Management Data"
```

Use the same explicit path every time. Starting the application with a different empty folder creates a separate, empty workspace; it does not mean the original data was deleted.

For a deliberately self-contained copy, start with `--portable`. This uses a `data` folder beside the executable and is useful on removable storage or in a folder that will be moved as one unit. The program folder must be writable.

## What the folder contains

| Path | Purpose |
| --- | --- |
| `app.db` | Main SQLite database containing project records and settings |
| `app.db-wal`, `app.db-shm` | Temporary SQLite files that can exist while the app is running |
| `app.log` | Startup, shutdown, and error log |
| `backups/backup-*.dpm-backup` | Complete, checksummed workspace archives created by the app |
| `backups/backup-*.db` | Legacy database-only snapshots, if any remain from an older release |
| `uploads/` | Files uploaded to project document sections |

Do not edit these files while the application is running.

## Automatic and manual workspace backups

On startup, the application creates at most one automatic complete workspace backup per day and retains the newest 14 backup files. Use **Settings → Back up now** to create an additional archive before an important change or upgrade.

Each `.dpm-backup` archive contains:

- A consistent snapshot of `app.db`
- Every file under `uploads/`
- A versioned manifest containing file sizes and SHA-256 checksums

The restore process validates the archive, manifest, sizes, and checksums before it replaces live data. It also creates a complete safety backup of the current workspace first.

Download important backups from Settings and copy them to protected storage away from the computer that holds the working data. Backups left only inside the active data folder do not protect against loss of that disk.

## Legacy database-only backups

Older releases created `.db` files containing database records but not uploaded document contents. These legacy backups are still accepted. Restoring one replaces database records while deliberately retaining the current `uploads/` folder.

If you must recover a legacy database and its uploaded documents on another computer, preserve both the `.db` backup and the matching `uploads/` directory. A database record cannot recreate a missing uploaded file.

## Restore a workspace

1. Confirm that you have selected the correct `.dpm-backup` file.
2. Open **Settings → Restore from a file**.
3. Choose the file and confirm the warning.
4. Return to the dashboard and check several projects.
5. Download an uploaded document to confirm the workspace is complete.

Restoring a complete workspace replaces the active database and uploads directory; it does not merge workspaces. If validation fails, the live workspace is left in place. A safety backup of the previous workspace is kept in the backups folder after a successful restore.

## Move to another computer

1. Create and download a complete `.dpm-backup` on the old computer.
2. Install or extract the same or a newer application version on the new computer.
3. Start the new application and open **Settings → Restore from a backup file**.
4. Select the transferred `.dpm-backup` and confirm the restore.
5. Confirm the data path under **Settings → About**.
6. Open representative projects and download at least one uploaded document.

Keep the old copy until the new installation has been verified.

## Recovery hygiene

- Test a recovery periodically instead of assuming a backup is usable.
- Keep more than one recovery generation.
- Copy critical `.dpm-backup` files outside the active data folder.
- Protect backups according to the sensitivity of the project information.
- Do not place confidential backups in a public source repository or unprotected file share.
- Include `app.log` when reporting a startup problem, but review it for sensitive paths before sharing.
