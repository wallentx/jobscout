#!/bin/sh
set -eu

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"

if [ -z "$repo_root" ]; then
	exit 1
fi

cd "$repo_root"

mode="${1:-staged}"
stamp_file=".checks"

if [ ! -f "$stamp_file" ]; then
	exit 1
fi

# shellcheck disable=SC1090
. "./$stamp_file"

if [ -z "${HEAD_SHA:-}" ] || [ -z "${CHECKED_TREE:-}" ]; then
	exit 1
fi

current_head="$(git rev-parse HEAD)"

case "$mode" in
	staged)
		current_tree="$(git write-tree)"
		;;
	worktree)
		current_tree="$(scripts/git-hooks/worktree-tree.sh)"
		;;
	*)
		printf 'usage: %s [staged|worktree]\n' "$0" >&2
		exit 2
		;;
esac

if [ "$current_head" = "$HEAD_SHA" ] && [ "$current_tree" = "$CHECKED_TREE" ]; then
	exit 0
fi

exit 1
