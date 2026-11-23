# Changelog

All notable changes to USB Backup are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the version
numbers follow [Semantic Versioning](https://semver.org/).

New changes go under **Unreleased**. To release, rename that section to the
new version with today's date, add an empty **Unreleased** above it, commit,
and push a tag `v<version>` (see "Releasing" in the README). The release
workflow refuses a tag without a matching section here and uses the section
as the release notes.

## [Unreleased]

### Fixed

- Windows: on a fresh installation the gear asked for a vault password that
  did not exist yet, so the first-time setup could not be reached. It now
  starts the setup; the new command `keys check` tells the two apart.
- Windows: the main window stayed frozen after the break-glass screen, and
  choosing an existing file for the break-glass key deleted that file.
- `keys init` without administrator rights left a vault behind that blocked
  every later attempt; it now checks first and undoes a failed run.

## [0.1.0] - 2026-11-23

First release.

### Added

- `usb-backup-cli` for Windows and Linux (amd64 and arm64) and the Windows
  window `usb-backup.exe`, which drives the CLI.
- Backups of source folders to registered USB disks as one zip per source
  per ISO week, with two disks taking turns (odd and even weeks) and 25
  weekly folders kept per disk by default.
- Every backup is verified after writing: SHA-256 per file, the zip
  structure and a read-back from the disk. Rotation only deletes old weekly
  folders after the new backup passed that check.
- Encryption with age and post-quantum keys (ML-KEM-768 + X25519). The
  computer only holds the public keys; a password vault and an offline
  break-glass key are needed to restore.
- Disks are recognised by a signed `disk-id.json` and their serial numbers.
  Settings, registrations and public keys are signed by the operator.
- Commands `run`, `verify`, `list`, `restore`, `status`, `enroll`, `disks`,
  `forget`, `keys` (init, passwd, rotate, rewrap, status), `config`,
  `install`, `uninstall`, `eject`, `version` and `help`, with `--json` output
  and fixed exit codes.
- Scheduling: Windows Task Scheduler (on USB insert, daily, weekly) and
  systemd timers plus a udev rule on Linux.
- Key rotation and re-wrapping of existing backups without re-encrypting the
  data.
- English and Dutch user texts.
- Windows setup (NSIS), `.deb`, `.rpm` and `.tar.gz` packages, and
  `SHA256SUMS` signed with Sigstore cosign.
- GPL-3.0-or-later license notices in all source files, `version` output and
  the window; `THIRD-PARTY-NOTICES.txt` for the bundled modules.

[Unreleased]: https://github.com/macsnoeren/backup-usb/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/macsnoeren/backup-usb/releases/tag/v0.1.0
