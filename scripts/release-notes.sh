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

# Prints the CHANGELOG.md section of one version, for the release notes.
# Fails when the section is missing or empty, so a tag
# cannot be released without its changelog entry. A pre-release tag
# (v1.2.3-rc1) uses the "Unreleased" section.
#
#   scripts/release-notes.sh v1.2.3 [CHANGELOG.md]
set -eu
[ $# -ge 1 ] || { echo "usage: $0 <tag> [changelog]" >&2; exit 64; }
version=${1#v}
file=${2:-CHANGELOG.md}
case $version in
  [0-9]*.[0-9]*.[0-9]*-*) section=Unreleased ;;
  [0-9]*.[0-9]*.[0-9]*) section=$version ;;
  *) echo "release-notes.sh: '$1' is not a version tag like v1.2.3" >&2; exit 64 ;;
esac

awk -v v="$section" '
  /^## / {
    if (found) exit
    if ($0 == "## " v) { found = 1; next }
  }
  found {
    if ($0 ~ /[^ \t]/) body = 1
    if (body) lines[++n] = $0
  }
  END {
    if (!found) { print "release-notes.sh: CHANGELOG has no section \"## " v "\"" > "/dev/stderr"; exit 2 }
    while (n > 0 && lines[n] !~ /[^ \t]/) n--
    if (n == 0) { print "release-notes.sh: the section of " v " is empty" > "/dev/stderr"; exit 2 }
    for (i = 1; i <= n; i++) print lines[i]
  }
' "$file"
