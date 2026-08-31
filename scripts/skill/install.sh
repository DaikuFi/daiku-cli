#!/bin/sh
set -eu

agent=${1:-}
case "$agent" in
  codex) agent_home=${CODEX_HOME:-"${HOME:?HOME is required}/.codex"} ;;
  claude) agent_home=${CLAUDE_HOME:-"${HOME:?HOME is required}/.claude"} ;;
  *) printf '%s\n' 'usage: install.sh codex|claude' >&2; exit 2 ;;
esac

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
source_dir="$root/skills/daiku"
skills_dir="$agent_home/skills"
target="$skills_dir/daiku"
stage=
backup=
backup_root=

cleanup() {
  if [ -n "$backup" ] && [ -d "$backup" ]; then
    if [ ! -e "$target" ] && [ ! -L "$target" ]; then
      if ! mv "$backup" "$target"; then
        printf '%s\n' "daiku skill installer: recovery copy remains at $backup" >&2
        return
      fi
    fi
  fi
  if [ -n "$backup_root" ]; then
    rm -rf "$backup_root"
  fi
  if [ -n "$stage" ]; then
    rm -rf "$stage"
  fi
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p "$skills_dir"

if [ -e "$target" ] || [ -L "$target" ]; then
  if [ ! -f "$target/SKILL.md" ] || ! grep -q '^name: daiku$' "$target/SKILL.md"; then
    printf '%s\n' "daiku skill installer: refusing to replace non-Daiku target at $target" >&2
    exit 1
  fi
fi

stage=$(mktemp -d "$skills_dir/.daiku.skill.XXXXXX")
cp -R "$source_dir/." "$stage"
if [ -e "$target" ] || [ -L "$target" ]; then
  backup_root=$(mktemp -d "$skills_dir/.daiku.skill.backup.XXXXXX")
  backup="$backup_root/daiku"
  mv "$target" "$backup"
  if mv "$stage" "$target"; then
    stage=
    rm -rf "$backup_root"
    backup=
    backup_root=
  else
    mv "$backup" "$target"
    backup=
    rmdir "$backup_root"
    backup_root=
    exit 1
  fi
else
  mv "$stage" "$target"
  stage=
fi
printf '%s\n' "Daiku skill installed for $agent at $target"
