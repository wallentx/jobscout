#!/bin/sh
set -eu

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"

if [ -z "$repo_root" ]; then
	exit 1
fi

cd "$repo_root"

git_index="$(git rev-parse --git-path index)"
tmp_index="$(mktemp "${TMPDIR:-/tmp}/jobscout-index.XXXXXX")"

trap 'rm -f "$tmp_index"' EXIT HUP INT TERM

if [ -f "$git_index" ]; then
	cp "$git_index" "$tmp_index"
fi

GIT_INDEX_FILE="$tmp_index" git add -A -- .
GIT_INDEX_FILE="$tmp_index" git write-tree
