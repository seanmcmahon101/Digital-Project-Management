# Security policy

## Supported versions

Security fixes are provided for the latest published release. Users should upgrade to the newest release after preserving a current backup.

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Older releases | No |

## Report a vulnerability privately

Please do not open a public issue for a suspected vulnerability.

Use [GitHub's private vulnerability reporting](https://github.com/seanmcmahon101/Digital-Project-Management/security/advisories/new) to describe the issue. Include:

- Affected version and operating system
- Reproduction steps or a minimal proof of concept
- Expected impact
- Any suggested remediation

Do not include real project data or credentials. You should receive an acknowledgement within seven days. Details will remain private while the report is assessed and a fix is prepared.

## Security model

Digital Project Management is designed as a single-user, local application. It listens on `127.0.0.1` and does not provide user accounts, authentication, TLS termination, or multi-user access controls.

Do not bind, proxy, tunnel, or publish the application to a local network or the internet. Doing so is outside the supported security model and can expose project data and write operations to other people.

The operator is responsible for:

- Protecting the computer account and storage containing application data
- Restricting access to database backups and uploaded documents
- Downloading releases only from this repository and verifying checksums
- Keeping the operating system and browser supported and updated
- Reviewing logs and exports before sharing them

Release binaries are not currently code-signed or notarised. The project publishes SHA-256 checksums so downloads can be checked for accidental corruption or replacement, but checksums are not a substitute for platform code signing.
