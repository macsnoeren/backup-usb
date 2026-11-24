# USB Backup

Verified, encrypted backups of your folders to USB disks that you register.
Plug in the disk that is due this week and the backup runs; when the screen
says *Everything is fine*, take the disk out again.

- Two disks take turns: **BACKUP1** in odd ISO weeks, **BACKUP2** in even
  weeks (configurable). Each disk keeps 25 weekly folders by default.
- Every backup is checked after writing (each file's SHA-256, the zip
  structure, and the file read back from the disk). Old folders are only
  deleted after the new backup passed that check.
- Backups are encrypted with [age](https://age-encryption.org) using
  post-quantum keys. The computer only holds the public keys, so automatic
  backups need no password. Restoring needs the vault password, or the
  break-glass key from your safe.
- A disk is recognised by a signed ID file and its serial numbers, not by
  its label, so nothing is ever written to (or deleted from) a random stick.

Windows: `usb-backup.exe` (window) and `usb-backup-cli.exe` (does all the
work). Linux: `usb-backup-cli`.

## Contents

1. [Install](#1-install)
2. [First-time setup](#2-first-time-setup)
3. [Daily use](#3-daily-use)
4. [Restore files](#4-restore-files)
5. [Emergency restore with the break-glass key](#5-emergency-restore-with-the-break-glass-key)
6. [Settings](#6-settings)
7. [Command reference and exit codes](#7-command-reference-and-exit-codes)
8. [Files and folders](#8-files-and-folders)
9. [Building and testing](#9-building-and-testing)
10. [License](#10-license)

## 1. Install

### Windows

Run `USB-Backup-Setup.exe` from the release page and follow the steps. It
installs to `C:\Program Files\USB Backup`, adds a start-menu entry and puts
`usb-backup-cli` in the PATH. At the end the USB Backup window opens.

Silent install: `USB-Backup-Setup.exe /S`. To roll out the same settings on
several PCs: `usb-backup-cli config import settings.json` (made with
`config export` on another PC; contains no keys).

Uninstalling (Apps and features) removes the program and its scheduled
tasks. Settings, registrations and your vault stay unless you tick them in
the uninstaller. Backups on the USB disks are never touched.

### Linux

```
sudo apt install ./usb-backup_<version>_amd64.deb     # Debian, Ubuntu
sudo dnf install ./usb-backup-<version>.x86_64.rpm    # Fedora, RHEL
# other distributions:
tar xzf usb-backup-<version>-linux-amd64.tar.gz && cd usb-backup-<version>-linux-amd64 && sudo ./install.sh
```

Recommended: `udisks2` (mount sticks without root) and `smartmontools`
(disk health). Nothing runs automatically after installing; you switch on a
schedule yourself (step 2.5).

## 2. First-time setup

You do this once, as the **administrator** (the "operator"). On Windows,
start USB Backup with *Run as administrator* (right-click), because the
settings folder `C:\ProgramData\USB Backup` is writable for administrators
only. On Linux use `sudo` for a system-wide setup in `/etc/usb-backup`, or
run as yourself for `~/.config/usb-backup`.

### With the window (Windows)

Right-click USB Backup in the Start menu, choose *Run as administrator* and
click the gear (⚙) at the top right. On a fresh installation there is no
password yet; you choose it now:

1. Choose a vault password (at least 12 characters, twice). Choose a **new**
   file for the **break-glass key**, then print it and keep the printout in a
   safe. Tick *I have stored the break-glass key safely* to continue.
2. The settings open. From now on the gear asks for this vault password.
3. **General and sources → Add folder…** for every folder you want to back
   up, with a short tag (for example `Data`, `Accounts`). The tag becomes the
   file name on the disk.
4. **Disks**: plug in the first disk, select it, name it `BACKUP1`, choose
   *odd weeks*, **Register**. Do the same with the second disk as `BACKUP2`,
   *even weeks*.
5. **Schedule**: leave *Start a backup when a registered disk is inserted*
   ticked and **Save**.

### With the command line (Windows or Linux)

```
# 2.1 Keys: a vault (password) and a break-glass key file, written once.
usb-backup-cli keys init --breakglass-out /media/safe-stick/breakglass.txt
#     (asks the new password twice; print the break-glass file and store it offline)

# 2.2 What to back up
usb-backup-cli config add-source --tag Data --path /home/maurice/Documents
usb-backup-cli config add-source --tag Photos --path /home/maurice/Pictures

# 2.3 Register both disks (each asks the vault password)
usb-backup-cli enroll /dev/sdb1 --name BACKUP1 --weeks odd      # Windows: enroll E: ...
usb-backup-cli enroll /dev/sdc1 --name BACKUP2 --weeks even
usb-backup-cli disks                                            # check

# 2.4 Try it without writing anything
usb-backup-cli run --dry-run

# 2.5 Switch on a schedule (pick one or both)
usb-backup-cli install --trigger usb                   # back up when a registered disk is plugged in
usb-backup-cli install --trigger daily --at 02:00      # or: weekly --day fri --at 18:00
```

On Linux, `install` needs root (it writes systemd units and a udev rule).
Instead of a schedule you can also use cron, for example every hour:
`0 * * * * /usr/bin/usb-backup-cli run --quiet-skip`. A run only does
something when the right disk is there and the last backup of this week is
older than `minIntervalHours` (20).

> **Important:** without the vault password **and** without the
> break-glass key, the backups cannot be read by anyone, including you.
> Test the break-glass key once (step 5) and keep it in a safe.

## 3. Daily use

Plug in the disk that is due. The window shows what happens:

| You see | What to do |
| --- | --- |
| *Insert USB disk BACKUP1* | Plug in BACKUP1. |
| *This week BACKUP1 is due. Remove BACKUP2 and insert BACKUP1* | Swap the disks. |
| *This disk is not registered* | It is not one of your backup disks. |
| *Backup in progress, do not remove the disk* (with progress) | Wait; **Cancel** stops safely. |
| *Checking the backup* | Wait. |
| *Everything is fine. You can remove the disk* | Click **Safely remove disk**, take it out, and put it away from the computer. |
| *Backup made, but 3 files were in use* (orange) | **Details** lists them; close those programs next time. |
| *Backup failed* (red) | Read the reason; old backups were not deleted. **Try again**. |
| *This disk must be updated by the administrator* | Backups still work; the administrator runs *Keys → Update disk now* after a key change. |

From the command line:

```
usb-backup-cli status            # connected disks: health, free space, folders, last backup
usb-backup-cli run               # make a backup now (to the disk that is due)
usb-backup-cli run --any-disk    # to any registered disk
usb-backup-cli verify            # check the newest backup again (without key: the encrypted file)
usb-backup-cli verify --all --full            # unpack and check everything (vault password), e.g. monthly
```

Each disk keeps 25 period folders; with two disks that is about a year of
history. Deleting old folders only ever happens after the new backup was
verified, and only for folders like `2026/47` that this program made.

## 4. Restore files

### With the window

Gear → log in → **Browse and restore**. Double-click a week and source on
the left; on the right double-click folders to open them (**Up** goes
back, **Search** finds names). Select files or folders (Ctrl/Shift-click)
and choose:

- **Restore to original place**: existing files are **never** overwritten;
  a restored copy of an existing file is called
  `name (restored 2026-11-22).ext`.
- **Save in…**: a folder you choose (default `Desktop\Restored`).

### With the command line

```
# What is on the disk?
usb-backup-cli list
# Files of one source in one week (asks the vault password)
usb-backup-cli list --week 2026-47 --tag Data --path Documents
usb-backup-cli list --week 2026-47 --tag Data --search invoice

# Restore one file, a folder, or everything of a source
usb-backup-cli restore --week 2026-47 --tag Data --file "Documents/invoice.pdf" --to ~/Restored
usb-backup-cli restore --week 2026-47 --tag Data --file Documents --original
usb-backup-cli restore --tag Data --to /tmp/everything          # newest week
```

Every restored file is checked against its SHA-256 before it gets its name.

## 5. Emergency restore with the break-glass key

Use this when the vault password is lost, the computer is gone, or to test
the key once a year.

On any computer with `usb-backup-cli`:

```
# Test: decrypt and check everything with the break-glass key
usb-backup-cli verify --all --identity /path/to/breakglass.txt
# Restore
usb-backup-cli restore --week 2026-47 --tag Data --to ./Restored --identity /path/to/breakglass.txt
```

Without this program at all, with only the standard
[age](https://age-encryption.org) tool (version 1.3 or newer, post-quantum
support):

```
age -d -i breakglass.txt X:\Backup\2026\47\Data.zip.age > Data.zip     # then unzip normally
age -d -i recovery.vault Data.zip.age > Data.zip                         # the vault works too (asks its password)
```

`status` warns as long as the break-glass key has never been tested.

**Lost the vault password?** With the break-glass key you can still read
everything. To continue making backups, set up new keys (`keys init
--force` on a fresh vault path), print the new break-glass key, and register
the disks again (`enroll ... --force`). Old backups stay readable with the
old break-glass key.

### Changing or replacing keys

```
usb-backup-cli keys passwd                 # new vault password; backups unchanged
usb-backup-cli keys rotate                 # new key (e.g. after a leaked password); new backups use it at once
usb-backup-cli keys rewrap                 # per disk: re-wrap the old backups for the new key
usb-backup-cli keys status                 # keys and the disks that still need re-wrapping
```

`keys rotate --full` also re-encrypts with new file keys (slower), for the
case that someone may have copied a backup with the old key.

## 6. Settings

`usb-backup-cli config show` lists all settings; change one with
`usb-backup-cli config set <key> <value>` (asks the vault password: settings
are signed by the operator, so a normal user cannot change them, and a
manual edit makes every run stop with exit code 7).

| Key | Default | Meaning |
| --- | --- | --- |
| `sources` | `C:\Data` on Windows | Folders with a tag; use `config add-source` / `remove-source` |
| `exclude` | `*.tmp`, `~$*`, `Thumbs.db`, `desktop.ini` | Skipped for every source (`*.tmp` matches a name, `logs/*.log` a path) |
| `keepPerDisk` | `25` | Folders kept per disk; `0` = never delete |
| `keepMinDays` | `0` | Never delete folders younger than this |
| `freeSpaceAction` | `stop` | Too little space: `stop`, or `deleteOldest` (only if that frees enough; the newest older backup always stays) |
| `schedule` | `isoWeek` | One folder per `isoWeek`, `day` or `month` |
| `folderPattern` | `{year}\{week}` | Folder layout, e.g. `{year}-{week}` or `{year}\{month}\{day}` |
| `rotation` | `oddEven` | `oddEven`, `roundRobin` over all disks, or `any` |
| `minIntervalHours` | `20` | Within the same folder, back up again only after this |
| `duplicateGuardMinutes` | `10` | Ignore a second trigger within this time |
| `compressionLevel` | `6` | Deflate 1–9; `0` stores everything |
| `storeExtensions` | jpg, png, mp4, mov, zip, 7z, rar, docx, xlsx, pptx | Stored without compression |
| `lockedFiles` | `skip` | Files in use: `skip` (warning) or `fail` |
| `encryption` | `auto` | `auto` (encrypt once keys exist), `age` (required), `none` |
| `requireHardwareSerial` | `false` | Refuse disks without a hardware serial number |
| `waitSeconds` | `30` | How long the trigger waits for the disk |
| `ejectAfter` | `true` | Eject the disk after a good backup, so it can be unplugged. Set it to `false` when the disk stays connected for a daily schedule |
| `language` | `en` | `en` or `nl` |
| `logMaxSizeMB`, `logKeep`, `logLevel` | `1`, `5`, `info` | Log rotation and detail |

## 7. Command reference and exit codes

```
usb-backup-cli help                 # all commands
usb-backup-cli help run             # flags of one command
```

| Command | Purpose |
| --- | --- |
| `run` | Back up (`--target`, `--any-disk`, `--tag`, `--force`, `--wait`, `--dry-run`, `--quiet-skip`, `--json`) |
| `verify` | Check backups (`--week`, `--tag`, `--all`, `--identity` or `--password-stdin`) |
| `list`, `restore` | Browse and restore (see section 4) |
| `status`, `disks`, `eject` | Disks |
| `enroll`, `forget` | Register or remove a disk |
| `keys init/passwd/rotate/rewrap/status` | Keys |
| `config show/set/add-source/remove-source/import/export` | Settings |
| `install`, `uninstall` | Schedule (`--trigger usb|daily|weekly`) |
| `version` | Version |

Passwords are never accepted as arguments or environment variables: give
`--password-stdin` (one line on stdin) or type it at the prompt.

| Exit code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Success with warnings (skipped files, disk health Warning) |
| 2 | No backup disk connected |
| 3 | Wrong disk for this period |
| 4 | Skipped: a backup was made recently |
| 5 | Another backup is running |
| 6 | Unknown disk (ID file, signature or serial numbers do not match) |
| 7 | Settings or registrations changed without the operator |
| 10 | Disk check failed (space, file system, not writable, keys missing) |
| 11 | Read or write error |
| 12 | Verification failed |
| 13 | Decryption or re-wrap failed (wrong key or password, damaged file) |
| 64 | Wrong usage |

For scheduled runs, 2 to 4 are normal (the disk is simply not there);
`--quiet-skip` makes them silent.

## 8. Files and folders

On the USB disk:

```
Backup\
├─ disk-id.json           signed ID of this disk (no secrets)
├─ last-backup.txt
├─ keys.json              folder patterns and key fingerprints per folder
└─ 2026\47\
   ├─ Data.zip.age        one archive per source (encrypted zip)
   ├─ Data.zip.age.sha256
   ├─ Data.index.age      encrypted index for browsing
   └─ report.txt          summary (no file names when encrypted)
```

On the computer: `C:\ProgramData\USB Backup\` or `/etc/usb-backup/`
(`~/.config/usb-backup/` without root) holds `usb-backup.json` (settings),
`usb-backup.disks.json` (registrations), both with `.sig` signatures,
`usb-backup.recipients` (public keys), `usb-backup.signing.pub` and the log
(`logs\` on Windows). The vault `recovery.vault` is per user in
`%APPDATA%\USB Backup\` or `~/.config/usb-backup/`.

## 9. Building and testing

Go 1.25 or newer, no cgo, no Docker:

```
make test test-integration test-e2e    # tests (fake disks and a fake clock)
make lint security                     # golangci-lint, govulncheck, gosec, race, fuzz
make build-windows build-linux         # dist/
make setup-windows packages-linux      # installers (needs makensis and nfpm)
```

### Releasing

Changes are listed in [`CHANGELOG.md`](CHANGELOG.md). To publish version
1.2.3:

1. In `CHANGELOG.md`, rename `## [Unreleased]` to `## [1.2.3] - YYYY-MM-DD`,
   add a new empty `## [Unreleased]` above it and update the links at the
   bottom.
2. `make release-check TAG=v1.2.3` shows the release notes and checks the
   GoReleaser configuration.
3. Commit, then `git tag -a v1.2.3 -m v1.2.3 && git push origin main v1.2.3`.

The tag starts `.github/workflows/release.yml`: it refuses a tag without a
dated changelog section, runs the tests on Linux and Windows, and then
publishes a GitHub release with the Windows setup (`USB-Backup-Setup.exe`),
`.deb`, `.rpm` and `.tar.gz` for Linux (amd64 and arm64), `.zip` files for
Windows, and `SHA256SUMS` signed with cosign. The changelog section becomes
the release notes. A tag such as `v1.2.3-rc1` is published as a pre-release
with the `Unreleased` notes. Windows files are code-signed when the secrets
`WINDOWS_CERT_PFX_BASE64` and `WINDOWS_CERT_PASSWORD` are set.

Design and build plan (in Dutch): `docs/DESIGN.md`, `docs/tasks.md`.
Security reviews: `docs/security-review.md`. Checks that need real hardware
or Windows: `docs/handmatige-tests.md`.

## 10. License

Copyright (C) 2026 Maurice Snoeren

USB Backup is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version (SPDX: `GPL-3.0-or-later`).

USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License in [`LICENSE`](LICENSE)
for more details.

Every source file starts with this notice. `usb-backup-cli version` and
*Help → About* in the window show it too. The dependencies keep their own
licenses, all compatible with the GPL: `filippo.io/age`, `filippo.io/hpke`,
`golang.org/x/crypto` and `golang.org/x/sys` (BSD-3-Clause) and windigo
(MIT). Their notices are in `THIRD-PARTY-NOTICES.txt` (`make notices`
regenerates it) and ship with every package.
