# Security review notes

Short notes per task (or phase) for work that touches secrets, crypto,
paths or deletion. Each entry: what was reviewed, what was found, what was
fixed.

## Phase 2 – zip and verification (T2.1–T2.6)

Reviewed by a separate review agent over `git diff fc16e2c..87320db`
(archive, verify, fsx, app), plus `make security`.

| # | Finding | Severity | Resolution |
| --- | --- | --- | --- |
| 1 | `.part` files were created with `O_CREATE|O_TRUNC` on a plain path: a symlink planted on the disk (`Backup/2026/47/Data.zip.part -> ~/.bashrc`) made the tool overwrite a file on the computer; a symlinked folder on the disk redirected the zip. | high | All disk writes now go through an `os.Root` on the backup folder (`fsx.OpenDiskRoot` refuses a backup folder that is a link), and files are created with `O_EXCL` after removing an old one (`fsx.CreateExclusive`). Test `TestDiskRootBlocksSymlinks`. |
| 2 | An unreadable sub folder vanished from the backup without a warning (`lockedFiles: skip`). | medium | `ReadDir`/`Lstat` errors are now recorded in `Skipped` (and fail with `lockedFiles: fail`); entries read before the error are kept. Test `TestUnreadableFolderIsReported`. |
| 3 | Names the restore refuses (control characters) were backed up without a warning; `:` was refused on Linux. | medium | `SafeRelOS` is platform aware; names that cannot be restored on this system are skipped and reported, names Windows cannot restore get a warning. |
| 4 | A new zip could end up next to the previous run's `.sha256`. | medium/low | The old `.sha256` is removed before the zip is renamed. |
| 5 | `SafeRel` accepted `COM¹`, trailing dots/spaces, `<>"|?*`, invalid UTF-8 and bidi controls. | low | Rejected (Windows-specific rules only for Windows). |
| 6 | A file swapped between listing and opening (for a link to `/dev/zero` or a FIFO) was read. | low | The open handle must be a regular file and `os.SameFile` as listed. |
| 7 | Files changing while being read went unnoticed. | low | Size and modification time are compared before and after; a warning is recorded. |
| 8 | `FollowSymlinks` does not follow links to folders. | low | Documented (prevents loops). |
| 9 | Crafted zip on the disk: no cap on total declared size when verifying without the writer's entries. | low | Accepted: reads are bounded per entry, `archive/zip` bounds memory by the file size, and the check can be cancelled. |
| 10 | Read-back can come from the page cache. | medium | Known open question in the design; listed under "Open punten". |

Found during the tasks themselves (before the review): damage to local
headers, data descriptors and the central directory was not detected by
`archive/zip`; now caught (test flips every byte). `CleanupParts` was
race-prone with symlinks; now root-scoped.

## Phases 3 and 4 – run, vault, keys, encryption, verify (T3.1–T4.8)

Reviewed by a separate review agent over `git diff 053536f..3110ad3`, with
exploit tests via `go test -overlay`, plus `make security`.

| # | Finding | Severity | Resolution |
| --- | --- | --- | --- |
| 1 | The write test (`CheckWritable`) and `CleanupParts` used plain paths: a planted `Backup/.usb-backup-write-test` symlink made `run` overwrite a file on the computer. | high | Both now work on the `os.Root` of the backup folder; the test file is created exclusively. Test with a planted link. |
| 2 | `freeSpaceAction: deleteOldest` could delete every old backup and still fail, and then claimed nothing was deleted. | medium-high | It now checks first that free space plus all deletable folders covers the need (else deletes nothing), always keeps the newest older backup, and reports how many folders were deleted. Tests for the hopeless case. |
| 3 | `disk-id.json` (on any stick) and the state files were opened through symlinks and FIFOs: a FIFO hung every run; a link read a file of the computer. | medium | `fsx.OpenRegular`: only regular files, opened with `O_NONBLOCK`, re-checked on the handle. Used for every read from a disk. Tests with a FIFO and a link. |
| 4 | Unwiped copies of `--identity` key files (`io.ReadAll`, `bufio.Scanner`). | low-medium | Read straight into a `secmem` buffer; lines split in place. |
| 5 | The break-glass file was synced after the vault and recipients were written. | low | Synced inside `KeysInit` before anything else is saved. |
| 6 | "Break-glass tested" was recorded even when nothing was decrypted. | low | Recorded only after at least one encrypted archive was unpacked with it. |
| 7 | Folders dated in the future (planted or wrong clock) counted toward `keepPerDisk` and became the default for verify. | low | `rotate.NotFuture` excludes them from rotation, space freeing and the default "newest" choice. |
| 8 | Plaintext archive of an earlier run in the same period stays when encryption is switched on; per-page `munlock`; 1 MiB vault lock. | low | Warning in the report; vault buffer sized to the file. Page-sharing of `munlock` is accepted (best effort, documented). |

Clean according to the review: secrets only cross into strings at the age
API boundary; no secrets in logs, errors or JSON; encryption fails closed;
rotation is root-scoped and pattern-strict; the stream verifier cannot
deadlock or panic; disk recognition requires registry, ID file and volume to
agree.

## Phases 5 and 6 – index, list, restore, signatures, enroll (T5.1–T6.5)

Reviewed by a separate review agent over `git diff 01e9841..ec46241`, with an
exploit test via `go test -overlay`.

| # | Finding | Severity | Resolution |
| --- | --- | --- | --- |
| 1 | For an encrypted archive, a missing `<tag>.index.age` fell back to a plaintext `<tag>.index.gz`, which anyone could plant: chosen file names in `list`, and known-content or link entries that passed restore. | high | `ReadIndex` only accepts the index type that matches the archive's encryption; otherwise restore falls back to the zip itself. Test with a planted index. |
| 2 | The symlink check was lexical only: earlier restored links could make `a/l/..` point outside the destination. | medium | Links with an absolute target or any `..` component are not restored. Test with `../notes`. |
| 3 | Anyone with the public keys (world-readable) can build an encrypted backup that verifies and restores. | medium (design) | Inherent to age (confidentiality, not authenticity) and to unattended backups. Documented under "Open punten"; restore never overwrites, so planted content cannot replace existing files. |
| 4 | `usb-backup.recipients` was unsigned, and with `encryption: auto` its removal silently switched to unencrypted backups. | medium | Signed with purpose `recipients` by `keys init`; once a signing key exists, `auto` requires encryption: removed → exit 10, changed → exit 7. Test. |
| 5 | Index up to 1 GiB decompressed could exhaust memory. | low | Limit 128 MiB. Restore size per file is bounded by the size in the index and the temporary file is removed when the hash fails. |
| 6 | Race between the existence check and the rename could overwrite a file created in between. | low | Hard link (fails if the name exists) and remove, falling back to check-and-rename only where hard links are not supported. |
| 7 | Restored modes allowed group/world write. | low | Masked with `&^ 0o022`. |
| 8 | `enroll` wrote the ID file before the registry; a refused chmod on FAT stopped it. | low | Registry first; read-only/hidden is best effort. |
| 9 | Replay of an old signed list, deleted `signing.pub`, terminal escapes in fallback names, vault check via stored metadata. | low/info | Replay and deletion need admin rights (documented boundary). Fallback names now pass `SafeRelOS`; the vault's public key is derived from its seed. |

Clean according to the review: path checks, hash check before naming,
offset bounds, canonical JSON (duplicates, escapes, numbers), purpose
separation, signature-before-data, ID file copy detection, operator key
required for enroll/forget/config set, key wiping.

## Phases 7–11 – platform code, scheduling, key rotation, JSON stream, GUI (T7.1–T11.11)

Reviewed by a separate review agent over `git diff 34d5123..46c5e2f`, with
exploit tests via `go test -overlay`.

| # | Finding | Severity | Resolution |
| --- | --- | --- | --- |
| 1 | `keys rewrap` opened the vault before taking the lock and saved its copy afterwards: a `keys rotate` (or `keys passwd`) in between was overwritten, losing the new private key. | medium | `keys rotate`, `keys rewrap` and `keys passwd` take the single-instance lock first and only then open the vault; the recipients file is written under the same lock. |
| 2 | Rewrap copied a bit-rotted payload and wrote a fresh `.sha256`, hiding the damage. | medium | Archives are only re-wrapped when the original matches its `.sha256` (hashed while copying); indexes are decrypted completely first; otherwise the file is left alone with exit 12. Test. |
| 3 | `/run/lock` is world-writable: any user could hold root's lock, or plant a FIFO that hung it. | medium-low | Root locks in `/run/usb-backup/` (root only), other users in their own runtime folder; opened with `O_NONBLOCK|O_NOFOLLOW` and must be a regular file of the process user. Tests with a FIFO and a symlink. On Windows another session can still create `Global\UsbBackup` first (accepted). |
| 4 | Root mounted any file system type the stick claimed, and mounts were never undone. | low-medium | Only an allowlist (vfat, exfat, ntfs, ntfs3, ext2/3/4, btrfs, xfs) is mounted, always with `-t`; every mount is undone at the end of the command, an unrecognised volume at once. Test. |
| 5 | The age header parser read whole lines before checking the size (1 GiB allocation). | low | `ReadSlice` on a 64 KiB buffer. Test with an 8 MiB line. |
| 6 | The mount point was parsed from `udisksctl` text (`BACKUP.` → `BACKUP`). | low | Taken from the kernel (`lsblk` + `mountinfo`) after mounting. |
| 7 | A misplaced `//nolint` comment swallowed PowerShell arguments. | low | Argument list fixed. |
| 8 | Newlines in `install --config` could add directives to a root unit; root units could start a program writable by others. | low | Control characters refused; as root, the program and `--config` folder (and their parents) must belong to root and not be writable by others. Test. |
| 9 | One click within 5 s after the GUI timeout revived the session; the password field became a Go string. | low | `Session.Password` ends an expired session itself; the password field is read with `GetWindowTextW` into a buffer that is wiped. |
| 10 | `eject` took no lock and could dismount a disk mid-job. | low | `eject` takes the lock. |

Clean according to the review: external programs (full paths, no shell,
argument lists, `--` before devices), disk-controlled text in udev rules and
Task XML, `nosuid,nodev,noexec` root mounts, header MAC and rewrap checks,
pending-disk bookkeeping, password handling over stdin, error handling.

## Fuzz finding – age header parser (found during the release work)

`make security` (FuzzParseHeader) found that `crypt.ParseHeader` accepted CR
and LF characters inside base64 stanza bodies and the MAC: `encoding/base64`
skips them, even in strict mode. age itself refuses them, so two different
byte strings parsed to the same header. Impact was small: rewrap hands the
re-encoded (canonical) header to `age.DecryptHeader` and the MAC still needs
the file key, so nothing could be forged, but a non-canonical header on a
crafted disk was silently normalised instead of refused.

| # | Finding | Severity | Resolution |
| --- | --- | --- | --- |
| 1 | CR/LF accepted in base64 of the age header (malleability). | low | `decodeB64` refuses CR and LF before decoding, as age does. Tests for a body line and the MAC line; the fuzz input is kept in `internal/crypt/testdata/fuzz` as a regression case. |

## First-time setup in the window (T11.8, found by the user)

Self-review of the diff (no separate agent).

| # | Finding | Severity | Resolution |
| --- | --- | --- | --- |
| 1 | The gear always asked the vault password, also without a vault: the first-time setup was unreachable. | functional | `keys check` (no password, public information only); setup only when no key file exists at all. Unreadable paths count as existing. Tests. |
| 2 | The window deleted an existing file chosen for the break-glass key, before `keys init` could refuse: possibly the only copy of the current key. | high (data loss) | Never deleted; an existing file is refused. Test. |
| 3 | `keys init` saved the vault before it found out that the settings folder was not writable. | medium | Writability is checked first; a failed run removes only the files it created. Tests. |
| 4 | After a password change the new session was not wiped when the settings closed. | low | The current session is ended. |

The window also read JSON answers from `output` while the CLI puts them in
`report`, so settings, language, timeout and the rewrap result were ignored
(the fake CLI in the tests had the wrong format). Fixed with `Event.Text`;
`TestRealCLI` now drives the real CLI.

Skipping the password gives no extra rights: a user who may not write the
settings folder is refused by the CLI, and existing keys are never replaced.

## Ejecting after a backup (self-review)

| # | Finding | Severity | Resolution |
| --- | --- | --- | --- |
| 1 | The disk was ejected while `Backup` was still open, so it always failed, and the report still said that the disk could be removed. | medium (risk of data loss) | Ejected after the disk folder is closed; on failure the message changes. Test that looks at the open files. |
| 2 | Removing the wrong device (a controller or hub) would be possible when asking the parent of a disk. | checked | Only the parent of the disk with the volume's own device number, and only if its ID is a USB device. Test. |

New libraries `setupapi.dll` and `cfgmgr32.dll` are loaded by full path. No
external programs, no disk-controlled text.

## T13: disks in turn, frequency (self-review)

| # | Finding | Severity | Resolution |
| --- | --- | --- | --- |
| 1 | Rotation now covers the folders of every schedule used on a disk. With `{year}\{week}` and `{year}\{month}\{day}`, deleting "week 11" would delete all days of November. | high (data loss) | A folder that matches two schedules or contains another period folder is never deleted; a pattern that clashes with an older one on the disk is refused before anything is written. The default patterns cannot clash. Tests. |
| 2 | Backups made before a change of `schedule` were invisible to `list`, `restore` and the re-wrapping of keys, so an old key could be dropped while old backups still needed it. | high | All patterns recorded on the disk are used. Tests. |
| 3 | The time of a backup comes from the disk and now orders the rotation. | checked | Only used within the period of the folder itself. Test. |
| 4 | `usb-backup.turns.json` is not signed and can be written by users. | low | It only orders registered (signed) disks. Size limit, strict parsing, opened below the log folder without following links out of it; damaged means every disk is accepted; numbers cannot overflow. Tests. |
| 5 | `keys.json` on a disk could list very many patterns, each one a walk through the folders. | low | At most 64 are used. |

No secrets, new dependencies or external programs.

## Release workflow (self-review)

| # | Finding | Severity | Resolution |
| --- | --- | --- | --- |
| 1 | The release was built with the Go version in `go.mod` (1.26.0), with known ways out of `os.Root`, also when following junctions on Windows. govulncheck stopped it. | high | The newest Go, as in CI; `go.mod` asks for 1.26.9 at least. |
| 2 | The tag is typed by a person and used in shell scripts and as a Git ref. | checked | Only an environment variable, quoted; checked against a strict pattern first (no spaces, newlines, options or patterns). Tests. Only on the default branch; an existing tag on another commit is never moved. |
| 3 | A release starts on its own after ci (`workflow_run`, which has more rights than a pull request). | checked | Only when ci passed for a push to main of this repository, not for a pull request (a fork can have a branch called main), and on the commit that ci tested. Only the newest dated version, without a tag and a release; whoever may push to main could already push a tag. The decision is in `scripts/plan-release.sh`, with tests; the version is checked before it becomes a workflow output. |
| 4 | The workflow can change the text of a release. | checked | Only an empty one, in a job of its own that runs nothing from the repository. |
| 5 | The workflow may push tags. | checked | Only the release job has write access, after the tests. It removes only a tag that it made itself in the same run, and only without a release. |

## Windows tests (self-review)

| # | Finding | Severity | Resolution |
| --- | --- | --- | --- |
| 1 | `secmem.Buffer.Wipe` after `Truncate` unlocked only what was left; with nothing left it indexed an empty slice and crashed on Windows (an empty password). The memory itself was wiped. | medium | Unlocks the whole buffer; `lock` and `unlock` accept an empty slice. Test. |
| 2 | Re-wrapping replaced a file that was still open, which Windows refuses on FAT and exFAT: the disk kept the old key. | medium | Closed before the rename. Test that looks at the open files. |
