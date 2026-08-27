# Getting started

This guide takes you from the GitHub release page to a running application. You do not need to install Go or set up a database.

## Before you begin

You need:

- A supported 64-bit Windows, macOS, or Linux computer
- A current web browser
- Permission to run an application and write to its data folder

Digital Project Management is a local application. It does not ask you to create an account and does not send your project data to a hosted service.

## 1. Choose the correct download

Go to the [latest release](https://github.com/seanmcmahon101/Digital-Project-Management/releases/latest) and choose the archive for your operating system and processor.

| Platform | Most common choice | Alternative |
| --- | --- | --- |
| Windows | `windows_amd64.zip` | `windows_arm64.zip` for Windows on ARM |
| macOS | `darwin_arm64.tar.gz` for Apple silicon | `darwin_amd64.tar.gz` for Intel Macs |
| Linux | `linux_amd64.tar.gz` | `linux_arm64.tar.gz` for ARM64 devices |

If you do not know which Windows processor you have, open **Settings → System → About → System type**. On macOS, open **Apple menu → About This Mac** and look for **Chip** or **Processor**.

## 2. Verify the download (optional)

Each release includes `checksums.txt`. A checksum confirms that the archive arrived unchanged.

On Windows PowerShell:

```powershell
Get-FileHash .\digital-project-management_1.2.3_windows_amd64.zip -Algorithm SHA256
```

On macOS:

```sh
shasum -a 256 digital-project-management_1.2.3_darwin_arm64.tar.gz
```

On Linux:

```sh
sha256sum digital-project-management_1.2.3_linux_amd64.tar.gz
```

Compare the result with the matching line in `checksums.txt` on the release page. The example version `1.2.3` should be replaced with the version you downloaded.

## 3. Extract and start

### Windows

1. Right-click the downloaded `.zip` file and select **Extract All**.
2. Open the extracted folder.
3. Double-click `digital-project-management.exe`.

The application opens a console window and then your browser. Keep the console window open while using the app, and press `Ctrl+C` there to stop it safely. Windows may display a SmartScreen prompt because the executable is not currently code-signed. Confirm the filename and that it came from this repository before choosing to run it.

### macOS

1. Double-click the `.tar.gz` archive to extract it.
2. Open Terminal and change to the extracted directory.
3. Run:

```sh
./digital-project-management
```

If macOS blocks the first launch because the binary is not notarised, confirm that you downloaded it from this repository, then use **System Settings → Privacy & Security → Open Anyway**. Do not disable Gatekeeper globally.

### Linux

Extract the archive, open a terminal in the extracted directory, and run:

```sh
./digital-project-management
```

If the executable permission was removed while copying the file, restore it once:

```sh
chmod +x digital-project-management
```

## 4. Confirm where your data is stored

Open **Settings** in the application. The **About** section shows the active data folder. Record this location before upgrading or moving the application.

New installations use a per-user configuration location chosen for the operating system. To protect existing installations, a legacy `data` folder beside the executable is reused when it already contains `app.db`. You can select an explicit folder with `--data`, or use `--portable` to keep data beside the executable.

## 5. Set up your workspace

In **Settings**:

1. Add your organisation name for reports.
2. Choose a currency symbol.
3. Set the default loaded hourly rate used by business-case calculations.
4. Optionally choose an organisation colour for the sidebar.

You can now add an idea or create a project directly. The [User guide](USER_GUIDE.md) describes the recommended workflow.

## Updating

1. Create a backup from **Settings**.
2. Note the active data-folder location.
3. Stop the existing application.
4. Download and extract the new release into a new program folder.
5. Start the new executable and confirm that it is using the expected data folder.

Never delete the data folder as part of an update. If the executable and data live in the same extracted directory, keep the old directory until you have confirmed that the new release sees your projects and uploaded documents.
