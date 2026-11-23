#!/bin/sh
# Remove the timers and the udev rule that "usb-backup-cli install" made.
# Only on removal, not on upgrade.
set -e
case "$1" in
  remove|purge|0)
    /usr/bin/usb-backup-cli uninstall --trigger all >/dev/null 2>&1 || true
    ;;
esac
exit 0
