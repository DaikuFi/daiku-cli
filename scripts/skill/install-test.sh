#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
case_root=$(mktemp -d "${TMPDIR:-/tmp}/daiku-skill-install-test.XXXXXX")
trap 'rm -rf "$case_root"' EXIT HUP INT TERM

CODEX_HOME="$case_root/codex" "$root/scripts/skill/install-codex.sh" >/dev/null
CLAUDE_HOME="$case_root/claude" "$root/scripts/skill/install-claude.sh" >/dev/null
cmp "$root/skills/daiku/SKILL.md" "$case_root/codex/skills/daiku/SKILL.md"
cmp "$root/skills/daiku/SKILL.md" "$case_root/claude/skills/daiku/SKILL.md"

printf '\nlocal edit\n' >> "$case_root/codex/skills/daiku/SKILL.md"
CODEX_HOME="$case_root/codex" "$root/scripts/skill/install-codex.sh" >/dev/null
cmp "$root/skills/daiku/SKILL.md" "$case_root/codex/skills/daiku/SKILL.md"

printf '\ninterrupted edit\n' >> "$case_root/codex/skills/daiku/SKILL.md"
cp "$case_root/codex/skills/daiku/SKILL.md" "$case_root/interrupted-skill.md"
mkdir -p "$case_root/bin"
cat > "$case_root/bin/mv" <<'EOF'
#!/bin/sh
case ${2:-} in
  */.daiku.skill.backup.*/daiku)
    "$DAIKU_TEST_REAL_MV" "$@"
    kill -TERM "$PPID"
    exit 0
    ;;
esac
exec "$DAIKU_TEST_REAL_MV" "$@"
EOF
chmod +x "$case_root/bin/mv"
DAIKU_TEST_REAL_MV=$(command -v mv)
export DAIKU_TEST_REAL_MV
if PATH="$case_root/bin:$PATH" CODEX_HOME="$case_root/codex" \
  "$root/scripts/skill/install-codex.sh" >/dev/null 2>&1; then
  printf '%s\n' 'interrupted installer unexpectedly succeeded' >&2
  exit 1
fi
cmp "$case_root/interrupted-skill.md" "$case_root/codex/skills/daiku/SKILL.md"
if find "$case_root/codex/skills" -maxdepth 1 \
  \( -name '.daiku.skill.*' -o -name '.daiku.skill.backup.*' \) | grep -q .; then
  printf '%s\n' 'interrupted installer left temporary paths behind' >&2
  exit 1
fi

mkdir -p "$case_root/foreign/skills/daiku"
printf '%s\n' 'not a Daiku skill' > "$case_root/foreign/skills/daiku/SKILL.md"
if CODEX_HOME="$case_root/foreign" "$root/scripts/skill/install-codex.sh" >/dev/null 2>&1; then
  printf '%s\n' 'installer replaced a foreign skill' >&2
  exit 1
fi
grep -q 'not a Daiku skill' "$case_root/foreign/skills/daiku/SKILL.md"
printf '%s\n' 'skill installer tests passed'
