#!/bin/sh
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
for u in usb-backup@.service usb-backup.service usb-backup.timer; do
  sed 's#/usr/bin/usb-backup-cli#/usr/local/bin/usb-backup-cli#' "$here/$u" > "/etc/systemd/system/$u"
  chmod 0644 "/etc/systemd/system/$u"
done
install -d /usr/local/share/man/man1
install -m 0644 "$here/usb-backup-cli.1" /usr/local/share/man/man1/usb-backup-cli.1
systemctl daemon-reload >/dev/null 2>&1 || true
echo "Installed /usr/local/bin/usb-backup-cli. Next: usb-backup-cli keys init, enroll, install --trigger usb|daily."
