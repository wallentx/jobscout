#!/bin/sh
set -eu

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"

if [ -z "$repo_root" ]; then
	exit 0
fi

cd "$repo_root"

staged_files="$(git diff --cached --name-only)"

if [ -z "$staged_files" ]; then
	exit 0
fi

requires_make_check=false
while IFS= read -r file; do
	case "$file" in
		*.go|go.mod|go.sum)
			requires_make_check=true
			break
			;;
	esac
done <<EOF
$staged_files
EOF

if [ "$requires_make_check" != "true" ]; then
	exit 0
fi

if scripts/git-hooks/validate-check-stamp.sh; then
	exit 0
fi

if scripts/git-hooks/validate-check-stamp.sh worktree; then
	cat >&2 <<'EOF'
Pre-commit check found unstaged or stale staged files.
Stage the files that were checked by make fix/check, then commit again.
EOF
	exit 1
fi

printf '%s\n' 'Running pre-commit check' >&2

if ! make check; then
	cat >&2 <<'EOF'
Pre-commit check failed.
Review the output above. Run make fix for automatic formatting/module updates, stage any changed files, then commit again.
EOF
	exit 1
fi

if scripts/git-hooks/validate-check-stamp.sh; then
	exit 0
fi

if scripts/git-hooks/validate-check-stamp.sh worktree; then
	cat >&2 <<'EOF'
Pre-commit check found unstaged or stale staged files.
Stage the files that were checked by make check, then commit again.
EOF
	exit 1
fi

cat >&2 <<'EOF'
Pre-commit check did not produce a valid check stamp.
Run make check, review any output, then commit again.
EOF

exit 1
