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

# Decides for .github/workflows/release.yml whether to release, and what. It
# prints three lines for GITHUB_OUTPUT: release=true or false, tag=v1.2.3, and
# notes=true when that release exists but has no text yet.
#
#   workflow_dispatch  the tag in INPUT_TAG, from the branch DEFAULT_BRANCH
#   push of a tag      that tag
#   workflow_run       (after ci) the newest version in CHANGELOG.md, if it has no tag
#                      and no release yet; "Unreleased" is never released
#
# A tag that does not fit CHANGELOG.md is an error (check-release.sh).
set -eu
file=${1:-CHANGELOG.md}
here=$(dirname "$0")
commit=$(git rev-parse HEAD) # what was checked out, and what ci tested

# answer release tag notes
answer() { printf 'release=%s\ntag=%s\nnotes=%s\n' "$1" "$2" "$3"; exit 0; }

fail() { echo "::error::$1" >&2; exit 1; }

# remote_tag prints the commit of a tag on the remote, or nothing. An
# annotated tag is listed twice; the last line is its commit.
remote_tag() {
  lines=$(git ls-remote origin "refs/tags/$1" "refs/tags/$1^{}") || fail "cannot read the tags of the repository"
  printf '%s\n' "$lines" | tail -n 1 | cut -f 1
}

if [ "$GITHUB_EVENT_NAME" = workflow_dispatch ]; then
  tag=${INPUT_TAG:-}
  sh "$here/check-release.sh" "$tag" "$file" >&2
  [ "$GITHUB_REF" = "refs/heads/$DEFAULT_BRANCH" ] ||
    fail "Releases are made from $DEFAULT_BRANCH, not from $GITHUB_REF."
  sha=$(remote_tag "$tag")
  if [ -n "$sha" ] && [ "$sha" != "$commit" ]; then
    fail "The tag $tag exists already, on another commit ($sha). Use the next version. If $tag was never released you can delete it first: git push origin :refs/tags/$tag"
  fi
elif [ "$GITHUB_EVENT_NAME" = push ] && [ "$GITHUB_REF_TYPE" = tag ]; then
  tag=$GITHUB_REF_NAME
  sh "$here/check-release.sh" "$tag" "$file" >&2
elif [ "$GITHUB_EVENT_NAME" != workflow_run ]; then
  fail "nothing is released for $GITHUB_EVENT_NAME of $GITHUB_REF"
else
  version=$(sed -n 's/^## \[\([0-9][0-9.]*\)\].*/\1/p' "$file" | head -n 1)
  tag=v$version
  if [ -z "$version" ]; then
    echo "::notice::CHANGELOG.md has no version yet." >&2
    answer false "" false
  fi
  sha=$(remote_tag "$tag")
  if gh release view "$tag" >/dev/null 2>&1; then
    echo "::notice::$tag, the newest version in CHANGELOG.md, is released already." >&2
    # A text that somebody wrote is never replaced.
    body=$(gh release view "$tag" --json body --jq .body) || fail "cannot read the release $tag"
    if [ -z "$(printf '%s' "$body" | tr -d '[:space:]')" ]; then
      sh "$here/release-notes.sh" "$tag" "$file" >/dev/null || fail "CHANGELOG.md has no usable section for $tag"
      answer false "$tag" true
    fi
    answer false "$tag" false
  elif [ -n "$sha" ]; then
    echo "::warning::$tag has a tag ($sha) but no release. Run this workflow by hand to release it." >&2
    answer false "$tag" false
  fi
  sh "$here/check-release.sh" "$tag" "$file" >&2
fi
answer true "$tag" false
