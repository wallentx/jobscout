#!/bin/sh
set -eu

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"

if [ -z "$repo_root" ]; then
	exit 0
fi

cd "$repo_root"

stamp_file=".checks"
head_sha="$(git rev-parse HEAD)"
checked_tree="$(scripts/git-hooks/worktree-tree.sh)"
source_target="${1:-manual}"
updated_at="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

tmp_file="${stamp_file}.tmp.$$"

cat >"$tmp_file" <<EOF
HEAD_SHA=$head_sha
CHECKED_TREE=$checked_tree
SOURCE_TARGET=$source_target
UPDATED_AT=$updated_at
EOF

mv "$tmp_file" "$stamp_file"
