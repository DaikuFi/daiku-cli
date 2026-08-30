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
stage="$skills_dir/.daiku.skill.$$"

cleanup() { rm -rf "$stage"; }
trap cleanup EXIT HUP INT TERM
mkdir -p "$skills_dir"

if [ -e "$target" ] || [ -L "$target" ]; then
  if [ ! -f "$target/SKILL.md" ] || ! grep -q '^name: daiku$' "$target/SKILL.md"; then
    printf '%s\n' "daiku skill installer: refusing to replace non-Daiku target at $target" >&2
    exit 1
  fi
fi

cp -R "$source_dir" "$stage"
if [ -e "$target" ] || [ -L "$target" ]; then
  backup="$skills_dir/.daiku.skill.backup.$$"
  mv "$target" "$backup"
  if mv "$stage" "$target"; then
    rm -rf "$backup"
  else
    mv "$backup" "$target"
    exit 1
  fi
else
  mv "$stage" "$target"
fi
printf '%s\n' "Daiku skill installed for $agent at $target"
