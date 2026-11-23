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

# Writes THIRD-PARTY-NOTICES.txt: the license of every module that is
# compiled into the Linux or Windows binaries. Their BSD and MIT licenses ask
# that the notice ships with the binaries. Run after changing go.mod.
set -eu
GO=${GO:-go}
out=${1:-THIRD-PARTY-NOTICES.txt}
self=$($GO list -m)
mods=$( { $GO list -deps -f '{{if .Module}}{{.Module.Path}}@{{.Module.Version}}{{end}}' ./...
          GOOS=windows $GO list -deps -f '{{if .Module}}{{.Module.Path}}@{{.Module.Version}}{{end}}' ./...; } |
        grep -v "^$self@" | sort -u)
{
  echo "USB Backup includes the following third-party software. Each part"
  echo "stays under its own license, reproduced below."
  for m in $mods; do
    dir=$($GO list -m -f "{{.Dir}}" "${m%@*}")
    lic=$(ls "$dir"/LICENSE* "$dir"/COPYING* 2>/dev/null | head -1)
    [ -n "$lic" ] || { echo "no license file for $m" >&2; exit 1; }
    echo
    echo "================================================================"
    echo "$m"
    echo "================================================================"
    echo
    cat "$lic"
  done
} > "$out"
