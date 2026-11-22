#!/bin/sh
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
