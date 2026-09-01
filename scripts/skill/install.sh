#!/bin/sh
set -eu
umask 077

current_uid=$(id -u)

directory_uid() {
  stat -c '%u' "$1" 2>/dev/null || stat -f '%u' "$1"
}

directory_mode() {
  stat -c '%a' "$1" 2>/dev/null || stat -f '%Lp' "$1"
}

validate_private_directory() {
  validation_directory=$1
  if [ -L "$validation_directory" ] || [ ! -d "$validation_directory" ]; then
    printf '%s\n' 'daiku skill installer: refusing unsafe parent directory' >&2
    return 1
  fi
  owner=$(directory_uid "$validation_directory") || {
    printf '%s\n' 'daiku skill installer: could not validate parent ownership' >&2
    return 1
  }
  mode=$(directory_mode "$validation_directory") || {
    printf '%s\n' 'daiku skill installer: could not validate parent permissions' >&2
    return 1
  }
  group_digit=$((mode / 10 % 10))
  other_digit=$((mode % 10))
  if [ "$owner" != "$current_uid" ] || [ $((group_digit & 2)) -ne 0 ] || [ $((other_digit & 2)) -ne 0 ]; then
    printf '%s\n' 'daiku skill installer: refusing unsafe parent directory' >&2
    return 1
  fi
}

ensure_private_directory() {
  ensured_directory=$1
  if [ -e "$ensured_directory" ] || [ -L "$ensured_directory" ]; then
    validate_private_directory "$ensured_directory"
    return
  fi
  ancestor=$(dirname "$ensured_directory")
  while [ ! -e "$ancestor" ] && [ ! -L "$ancestor" ]; do
    next=$(dirname "$ancestor")
    [ "$next" != "$ancestor" ] || break
    ancestor=$next
  done
  validate_private_directory "$ancestor"
  mkdir -p "$ensured_directory"
  chmod 700 "$ensured_directory"
  validate_private_directory "$ensured_directory"
}

agent=${1:-}
case "$agent" in
  codex) agent_home=${CODEX_HOME:-"${HOME:?HOME is required}/.codex"} ;;
  claude) agent_home=${CLAUDE_HOME:-"${HOME:?HOME is required}/.claude"} ;;
  *) printf '%s\n' 'usage: install.sh codex|claude' >&2; exit 2 ;;
esac

root=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
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
ensure_private_directory "$agent_home"
ensure_private_directory "$skills_dir"

if [ -e "$target" ] || [ -L "$target" ]; then
  validate_private_directory "$target"
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
