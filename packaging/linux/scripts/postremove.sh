#!/bin/sh
# "apt purge" also removes the configuration folder. The backups on the
# USB disks are never touched.
set -e
if [ "$1" = "purge" ]; then
  rm -rf /etc/usb-backup
fi
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload >/dev/null 2>&1 || true
fi
exit 0
