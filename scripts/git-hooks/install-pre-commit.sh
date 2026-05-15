#!/bin/sh
set -eu

marker="jobscout managed pre-commit hook"
repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"

if [ -z "$repo_root" ]; then
	exit 0
fi

hook_path="$(git rev-parse --git-path hooks/pre-commit)"
hook_dir="$(dirname "$hook_path")"

if [ -e "$hook_path" ] && ! grep -q "$marker" "$hook_path" 2>/dev/null; then
	exit 0
fi

mkdir -p "$hook_dir"
tmp_hook="${hook_path}.tmp.$$"

cat >"$tmp_hook" <<'EOF'
#!/bin/sh
# jobscout managed pre-commit hook
set -eu

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "$repo_root" ]; then
	exit 0
fi

exec "$repo_root/scripts/git-hooks/pre-commit.sh" "$@"
EOF

chmod +x "$tmp_hook"
mv "$tmp_hook" "$hook_path"
