#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
exec "$root/scripts/skill/install.sh" codex
