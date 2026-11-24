#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Maurice Snoeren
#
# This file is part of USB Backup.
#
# USB Backup is free software: you can redistribute it and/or modify it under
# the terms of the GNU General Public License as published by the Free Software
# Foundation, either version 3 of the License, or (at your option) any later
# version.
#
# USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
# WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
# PARTICULAR PURPOSE. See the GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License along with
# USB Backup. If not, see <https://www.gnu.org/licenses/>.

# Installs usb-backup-cli from the .tar.gz on distributions without .deb or
# .rpm. Run as root:  sudo ./install.sh   (uninstall: sudo ./install.sh --remove)
# Nothing runs automatically afterwards.
set -eu
if [ "$(id -u)" -ne 0 ]; then
  echo "install.sh: run as root (sudo ./install.sh)" >&2
  exit 1
fi
here=$(cd "$(dirname "$0")" && pwd)
if [ "${1:-}" = "--remove" ]; then
  /usr/local/bin/usb-backup-cli uninstall --trigger all >/dev/null 2>&1 || true
  for f in /usr/local/bin/usb-backup-cli /etc/systemd/system/usb-backup@.service \
           /etc/systemd/system/usb-backup.service /etc/systemd/system/usb-backup.timer \
           /usr/local/share/man/man1/usb-backup-cli.1; do
    if [ -f "$f" ]; then rm -f "$f"; fi
  done
  systemctl daemon-reload >/dev/null 2>&1 || true
  echo "Removed. /etc/usb-backup and the backups on the disks were kept."
  exit 0
fi
install -m 0755 "$here/usb-backup-cli" /usr/local/bin/usb-backup-cli
install -d -m 0755 -o root -g root /etc/usb-backup
u=usb-backup@.service
sed 's#/usr/bin/usb-backup-cli#/usr/local/bin/usb-backup-cli#' "$here/$u" > "/etc/systemd/system/$u"
chmod 0644 "/etc/systemd/system/$u"
install -d /usr/local/share/man/man1
install -m 0644 "$here/usb-backup-cli.1" /usr/local/share/man/man1/usb-backup-cli.1
systemctl daemon-reload >/dev/null 2>&1 || true
echo "Installed /usr/local/bin/usb-backup-cli. Next: usb-backup-cli keys init, enroll, install."
