#!/bin/sh
# Signs the Windows executables and the setup with osslsigncode, when a
# code-signing certificate is available; otherwise it prints a warning and
# succeeds (unsigned builds work, with a SmartScreen warning on first start).
#
#   WINDOWS_CERT_PFX_BASE64   the PKCS#12 certificate, base64 (CI secret)
#   WINDOWS_CERT_PASSWORD     its password (CI secret)
#   TIMESTAMP_URL             optional, default http://timestamp.digicert.com
#
#   scripts/sign-windows.sh dist/usb-backup.exe dist/usb-backup-cli.exe dist/USB-Backup-Setup.exe
set -eu

if [ -z "${WINDOWS_CERT_PFX_BASE64:-}" ]; then
  msg="No code-signing certificate configured (WINDOWS_CERT_PFX_BASE64): Windows files stay unsigned."
  if [ -n "${GITHUB_ACTIONS:-}" ]; then echo "::warning::$msg"; else echo "warning: $msg" >&2; fi
  exit 0
fi
command -v osslsigncode >/dev/null 2>&1 || { echo "sign-windows.sh: osslsigncode is not installed" >&2; exit 1; }

# The certificate and its password only live in a private temporary folder
# that is removed on exit; the password is read from a file, never passed
# as an argument (visible in the process list).
umask 077
work=$(mktemp -d)
cleanup() {
  if [ -f "$work/cert.pfx" ]; then shred -u "$work/cert.pfx" 2>/dev/null || rm -f "$work/cert.pfx"; fi
  if [ -f "$work/pass" ]; then shred -u "$work/pass" 2>/dev/null || rm -f "$work/pass"; fi
  rmdir "$work" 2>/dev/null || true
}
trap cleanup EXIT INT TERM
printf '%s' "$WINDOWS_CERT_PFX_BASE64" | base64 -d > "$work/cert.pfx"
printf '%s' "${WINDOWS_CERT_PASSWORD:-}" > "$work/pass"

ts="${TIMESTAMP_URL:-http://timestamp.digicert.com}"
for f in "$@"; do
  if [ "$ts" = "none" ]; then
    # Local tests: no timestamp server.
    osslsigncode sign -pkcs12 "$work/cert.pfx" -readpass "$work/pass" -n "USB Backup" -h sha256 -in "$f" -out "$f.signed"
  else
    osslsigncode sign -pkcs12 "$work/cert.pfx" -readpass "$work/pass" -n "USB Backup" -h sha256 -t "$ts" -in "$f" -out "$f.signed"
  fi
  mv "$f.signed" "$f"
  osslsigncode verify -in "$f" >/dev/null || echo "note: $f is signed but the chain is not trusted on this machine" >&2
  echo "signed $f"
done
