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
