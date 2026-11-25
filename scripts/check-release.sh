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

# Checks that a tag can be released: it is a version tag, CHANGELOG.md has its
# section, that section is the newest one, and nothing is left under
# "Unreleased" that would be in the release but not in its notes. A
# pre-release tag (v1.2.3-rc1) uses the "Unreleased" section instead.
#
#   scripts/check-release.sh v1.2.3 [CHANGELOG.md]
set -eu
[ $# -ge 1 ] || { echo "usage: $0 <tag> [changelog]" >&2; exit 64; }
tag=$1
file=${2:-CHANGELOG.md}

fail() {
  if [ -n "${GITHUB_ACTIONS:-}" ]; then echo "::error::$1"; else echo "check-release.sh: $1" >&2; fi
  exit 2
}

# First the characters: grep looks at lines, and a tag could hold several.
case $tag in
  *[!0-9A-Za-z.-]*) fail "this is not a version tag like v1.2.3 or v1.2.3-rc1" ;;
esac
n='(0|[1-9][0-9]*)'
printf '%s\n' "$tag" | grep -Eq "^v$n\.$n\.$n(-(alpha|beta|rc)\.?[0-9]+)?\$" ||
  fail "'$tag' is not a version tag like v1.2.3 or v1.2.3-rc1"

sh "$(dirname "$0")/release-notes.sh" "$tag" "$file" >/dev/null ||
  fail "CHANGELOG.md has no usable section for $tag"

case $tag in *-*) exit 0 ;; esac

newest=$(sed -n 's/^## \[\([0-9][^]]*\)\].*/\1/p' "$file" | head -n 1)
[ "v$newest" = "$tag" ] ||
  fail "$tag is not the newest version in CHANGELOG.md, that is v$newest"
left=$(awk '/^## \[/ { in_unreleased = ($0 ~ /^## \[Unreleased\]/); next } in_unreleased && /^[-*] / { n++ } END { print n + 0 }' "$file")
[ "$left" -eq 0 ] ||
  fail "CHANGELOG.md lists $left changes under Unreleased; move them to the section of a version first"
