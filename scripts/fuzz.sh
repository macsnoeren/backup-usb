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

# Runs every fuzz test in the module for $FUZZTIME (default 30s) each.
set -eu
GO=${GO:-go}
FUZZTIME=${FUZZTIME:-30s}
for pkg in $($GO list ./...); do
	for name in $($GO test -list '^Fuzz' "$pkg" 2>/dev/null | grep '^Fuzz' || true); do
		echo "fuzz $pkg $name ($FUZZTIME)"
		$GO test -run '^$' -fuzz "^${name}\$" -fuzztime "$FUZZTIME" "$pkg"
	done
done
