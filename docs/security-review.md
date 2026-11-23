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
