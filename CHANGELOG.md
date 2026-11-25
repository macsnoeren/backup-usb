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

## [0.3.0] - 2026-11-25

### Added

- Windows: USB Backup can keep running next to the clock when the window is
  closed (*General and sources*, off by default). The window comes forward
  when a registered disk is plugged in; any other stick does nothing.
- Windows: it can start there, hidden, when a user logs in (a separate tick
  box, off by default).

### Changed

- Windows: "another backup is running" is no longer shown as a failed backup.

- Windows: the settings ask for administrator rights when they are needed, and
  open in a window of their own. *Run as administrator* is not needed any more,
  and backups keep running without those rights.

### Fixed

- Forgetting a disk, or any other change of the settings, without
  administrator rights said "writing to the backup disk failed". It now says
  that the settings folder cannot be changed, and why.
- `keys rotate` without administrator rights left a new key in the vault that
  was not used.

## [0.2.0] - 2026-11-25

### Added

- Any number of disks: they take turns, and the one that was used longest ago
  is next. The window and `disks` show whose turn it is.
- How often a backup is made can be chosen under *General and sources*: every
  week, day or month, or every time a disk is plugged in.
- A program icon, for the window, the programs and the setup.
- The window shows how many backups are on the disk and how much space is
  free, also after the disk was ejected; `status` shows the same.
- Windows: a menu bar with *File → Settings…*, *File → Exit* and
  *Help → About USB Backup…*. The About box needs no password and shows the
  version, platform, settings folder, whether keys are set up, and the
  license.

### Changed

- Odd and even weeks are gone: settings with `oddEven` or `roundRobin` now use
  `inTurn`, and `enroll --weeks` does not exist any more. A week without a
  backup, and week 53, no longer use a disk twice in a row.
- After a change of `schedule` the older backups on a disk can still be listed
  and restored, and count for `keepPerDisk`.
- A `folderPattern` whose folders could be mixed up with older backups on the
  disk is refused.
- `ejectAfter` is now on by default: after a good backup the disk is ejected
  and the window says that it can be unplugged, or that it is still in use.
- Windows: password windows have the focus in the password field; Enter
  presses OK and Escape closes the window.

### Removed

- Daily and weekly tasks, with the Schedule screen. A backup starts when a
  registered disk is plugged in (a tick box under *General and sources* that
  shows the real state, or `install`). The Windows setup removes tasks that an
  older version made.

### Fixed

- `THIRD-PARTY-NOTICES.txt` lacked the license of Go itself.
- Windows: an empty password crashed the program instead of being refused.
- Windows: updating a disk after replacing the key (`keys rewrap`) failed on
  FAT and exFAT disks, because the old file was still open.
- Built with a Go version without known holes in `os.Root` (GO-2026-6604,
  GO-2026-4970, GO-2026-4602); Go 1.26.9 is now the minimum.
- Windows: *Open log* looked in the wrong folder.
- Windows: cancelling *Save in…* when restoring still restored the files, to
  `Restored` on the desktop. That folder now has its own button.
- `disks` printed `%!(EXTRA …)` after disks that are not registered.
- `ejectAfter` did not work: the disk was ejected while the backup folder was
  still open. On Windows, hard disks are now ejected the way "Safely remove
  hardware" does, which needs no administrator rights.
- Windows: on a fresh installation the gear asked for a vault password that
  did not exist yet, so the first-time setup could not be reached. It now
  starts the setup; the new command `keys check` tells the two apart.
- Windows: the main window stayed frozen after the break-glass screen, and
  choosing an existing file for the break-glass key deleted that file.
- Windows: the settings screens ignored the real settings (folders, language
  and timeout were read from the wrong field of the CLI's answer).
- Windows: drop-down lists opened without any items, so only the first choice
  could be used; the language list always showed English.
- Cancelling while the backup was being checked was reported as a failed
  check (exit 12) instead of cancelled.
- A source folder that does not exist (such as the default `C:\Data`) blocked
  saving every setting and adding or removing other folders.
- Windows: switching off "start a backup when a disk is inserted" failed when
  that task did not exist.
- `keys init` without administrator rights left a vault behind that blocked
  every later attempt; it now checks first and undoes a failed run.

## [0.1.0] - 2026-11-23

First version, not published.

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

[Unreleased]: https://github.com/macsnoeren/backup-usb/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/macsnoeren/backup-usb/releases/tag/v0.2.0
[0.1.0]: https://github.com/macsnoeren/backup-usb/commits/30d0802
