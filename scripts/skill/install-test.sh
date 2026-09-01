#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
case_root=$(mktemp -d "${TMPDIR:-/tmp}/daiku-skill-install-test.XXXXXX")
trap 'rm -rf "$case_root"' EXIT HUP INT TERM

(umask 000; CODEX_HOME="$case_root/codex" "$root/scripts/skill/install-codex.sh" >/dev/null)
(umask 000; CLAUDE_HOME="$case_root/claude" "$root/scripts/skill/install-claude.sh" >/dev/null)
cmp "$root/skills/daiku/SKILL.md" "$case_root/codex/skills/daiku/SKILL.md"
cmp "$root/skills/daiku/SKILL.md" "$case_root/claude/skills/daiku/SKILL.md"
cmp "$root/skills/daiku/integrity.json" "$case_root/codex/skills/daiku/integrity.json"
cmp "$root/skills/daiku/integrity.json" "$case_root/claude/skills/daiku/integrity.json"
cmp "$root/skills/daiku/references/commands.json" "$case_root/codex/skills/daiku/references/commands.json"
cmp "$root/skills/daiku/references/commands.json" "$case_root/claude/skills/daiku/references/commands.json"
cmp "$root/skills/daiku/references/safety.md" "$case_root/codex/skills/daiku/references/safety.md"
cmp "$root/skills/daiku/references/safety.md" "$case_root/claude/skills/daiku/references/safety.md"
cmp "$root/skills/daiku/references/workflows.md" "$case_root/codex/skills/daiku/references/workflows.md"
cmp "$root/skills/daiku/references/workflows.md" "$case_root/claude/skills/daiku/references/workflows.md"

directory_mode() {
  stat -c '%a' "$1" 2>/dev/null || stat -f '%Lp' "$1"
}

assert_private() {
  mode=$(directory_mode "$1")
  case $mode in
    ''|*[!0-9]*)
      printf '%s\n' 'directory mode helper did not return one numeric value' >&2
      exit 1
      ;;
  esac
  group_digit=$((mode / 10 % 10))
  other_digit=$((mode % 10))
  if [ $((group_digit & 7)) -ne 0 ] || [ $((other_digit & 7)) -ne 0 ]; then
    printf '%s\n' 'installer created a non-private directory under umask 000' >&2
    exit 1
  fi
}

for directory in \
  "$case_root/codex" "$case_root/codex/skills" "$case_root/codex/skills/daiku" \
  "$case_root/claude" "$case_root/claude/skills" "$case_root/claude/skills/daiku"; do
  assert_private "$directory"
done
if find "$case_root/codex/skills/daiku" "$case_root/claude/skills/daiku" -perm -022 -print | grep -q .; then
  printf '%s\n' 'installer created group/world-writable skill content under umask 000' >&2
  exit 1
fi

mkdir -p "$case_root/unsafe/skills/daiku"
chmod 700 "$case_root/unsafe" "$case_root/unsafe/skills/daiku"
printf '%s\n' 'name: daiku' 'unsafe destination sentinel' > "$case_root/unsafe/skills/daiku/SKILL.md"
cp "$case_root/unsafe/skills/daiku/SKILL.md" "$case_root/unsafe-before"
chmod 777 "$case_root/unsafe/skills"
if CODEX_HOME="$case_root/unsafe" "$root/scripts/skill/install-codex.sh" >/dev/null 2>&1; then
  printf '%s\n' 'installer accepted a group/world-writable existing parent' >&2
  exit 1
fi
cmp "$case_root/unsafe-before" "$case_root/unsafe/skills/daiku/SKILL.md"

mkdir -p "$case_root/symlink-home" "$case_root/real-skills/daiku"
chmod 700 "$case_root/symlink-home" "$case_root/real-skills" "$case_root/real-skills/daiku"
printf '%s\n' 'name: daiku' 'symlink destination sentinel' > "$case_root/real-skills/daiku/SKILL.md"
cp "$case_root/real-skills/daiku/SKILL.md" "$case_root/symlink-before"
ln -s "$case_root/real-skills" "$case_root/symlink-home/skills"
if CLAUDE_HOME="$case_root/symlink-home" "$root/scripts/skill/install-claude.sh" >/dev/null 2>&1; then
  printf '%s\n' 'installer accepted a symlinked existing parent' >&2
  exit 1
fi
cmp "$case_root/symlink-before" "$case_root/real-skills/daiku/SKILL.md"

mkdir -p "$case_root/runtime-home" "$case_root/runtime-config"
chmod 700 "$case_root/runtime-home" "$case_root/runtime-config"
(cd "$root" && go build -o "$case_root/daiku" ./cmd/daiku)
PATH="$case_root:$PATH" HOME="$case_root/runtime-home" XDG_CONFIG_HOME="$case_root/runtime-config" \
  CODEX_HOME="$case_root/codex" CLAUDE_HOME="$case_root/claude" \
  "$case_root/daiku" doctor --json > "$case_root/doctor.json"
grep -q '"code":"codex_skill_ok"' "$case_root/doctor.json"
grep -q '"code":"claude_skill_ok"' "$case_root/doctor.json"

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
