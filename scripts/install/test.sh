#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
case_root=$(mktemp -d "${TMPDIR:-/tmp}/daiku-installer-test.XXXXXX")
current_test=setup
finish() {
  status=$?
  trap - EXIT HUP INT TERM
  rm -rf "$case_root"
  if [ "$status" -ne 0 ]; then printf 'installer tests failed: %s\n' "$current_test" >&2; fi
  exit "$status"
}
trap finish EXIT
trap 'exit 130' HUP INT TERM

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1"
  else
    shasum -a 256 "$1"
  fi
}

make_case() {
  name=$1
  dir="$case_root/$name"
  mkdir -p "$dir/bin" "$dir/releases" "$dir/install"
  printf '#!/bin/sh\necho new\n' > "$dir/releases/daiku"
  chmod +x "$dir/releases/daiku"
  tar -czf "$dir/releases/daiku_1.2.3-rc.1_linux_amd64.tar.gz" -C "$dir/releases" daiku
  sha256_file "$dir/releases/daiku_1.2.3-rc.1_linux_amd64.tar.gz" | awk '{ print $1 "  daiku_1.2.3-rc.1_linux_amd64.tar.gz" }' > "$dir/releases/checksums.txt"
  : > "$dir/releases/checksums.txt.sig"; : > "$dir/releases/checksums.txt.pem"
  # shellcheck disable=SC2016 # Deliberately generate a mock whose variables expand when invoked.
  printf '#!/bin/sh\ncase "$1" in -s) echo Linux;; -m) echo x86_64;; *) exit 2;; esac\n' > "$dir/bin/uname"
  # shellcheck disable=SC2016 # Deliberately generate a mock whose variables expand when invoked.
  printf '%s\n' '#!/bin/sh' 'set -eu' \
    'identity= issuer=' \
    'while [ "$#" -gt 0 ]; do case "$1" in --certificate-identity) identity=$2; shift 2;; --certificate-oidc-issuer) issuer=$2; shift 2;; *) shift;; esac; done' \
    '[ "$identity" = "https://github.com/DaikuFi/daiku-cli/.github/workflows/release.yml@refs/heads/main" ]' \
    '[ "$issuer" = "https://token.actions.githubusercontent.com" ]' \
    'exit "${COSIGN_EXIT:-0}"' > "$dir/bin/cosign"
  # shellcheck disable=SC2016 # Deliberately generate a mock whose variables expand when invoked.
  printf '#!/bin/sh\nurl=""; out=""; while [ "$#" -gt 0 ]; do case "$1" in --output) out=$2; shift 2;; http*) url=$1; shift;; *) shift;; esac; done; cp "$FIXTURE_DIR/${url##*/}" "$out"\n' > "$dir/bin/curl"
  chmod +x "$dir/bin/"*
  printf '%s\n' "$dir"
}

update_checksum() {
  dir=$1
  sha256_file "$dir/releases/daiku_1.2.3-rc.1_linux_amd64.tar.gz" | awk '{ print $1 "  daiku_1.2.3-rc.1_linux_amd64.tar.gz" }' > "$dir/releases/checksums.txt"
}

for valid in v0.0.1-rc.1 v1.2.3-alpha v1.2.3-0.alpha-1; do
  current_test="valid version $valid"
  "$root/scripts/install/daiku.sh" --validate-version "$valid"
done
for invalid in '' v1.2.3 v01.2.3-rc.1 v1.02.3-rc.1 v1.2.03-rc.1 v1.2.3- v1.2.3-01 v1.2.3-rc..1 v1.2.3-rc_1; do
  current_test="invalid version ${invalid:-empty}"
  if "$root/scripts/install/daiku.sh" --validate-version "$invalid"; then echo "malformed version accepted: $invalid" >&2; exit 1; fi
done

run_installer() {
  dir=$1
  PATH="$dir/bin:$PATH" FIXTURE_DIR="$dir/releases" DAIKU_INSTALL_DIR="$dir/install" \
    DAIKU_VERSION=v1.2.3-rc.1 "$root/scripts/install/daiku.sh"
}

current_test=success
dir=$(make_case success)
run_installer "$dir" >/dev/null
[ "$("$dir/install/daiku")" = new ]

current_test=bad-signature
dir=$(make_case bad-signature)
if COSIGN_EXIT=1 run_installer "$dir" >/dev/null 2>&1; then echo 'invalid signature was accepted' >&2; exit 1; fi
unset COSIGN_EXIT
[ ! -e "$dir/install/daiku" ]

current_test=bad-identity
dir=$(make_case bad-identity)
if DAIKU_COSIGN_IDENTITY=https://example.invalid/workflow run_installer "$dir" >/dev/null 2>&1; then echo 'wrong signing identity was accepted' >&2; exit 1; fi
unset DAIKU_COSIGN_IDENTITY

current_test=bad-issuer
dir=$(make_case bad-issuer)
if DAIKU_COSIGN_ISSUER=https://example.invalid/issuer run_installer "$dir" >/dev/null 2>&1; then echo 'wrong signing issuer was accepted' >&2; exit 1; fi
unset DAIKU_COSIGN_ISSUER

current_test=bad-checksum
dir=$(make_case bad-checksum)
printf '0  daiku_1.2.3-rc.1_linux_amd64.tar.gz\n' > "$dir/releases/checksums.txt"
if run_installer "$dir" >/dev/null 2>&1; then echo 'invalid checksum was accepted' >&2; exit 1; fi

current_test=duplicate-checksum
dir=$(make_case duplicate-checksum)
cat "$dir/releases/checksums.txt" >> "$dir/releases/checksums.txt.duplicate"
cat "$dir/releases/checksums.txt" >> "$dir/releases/checksums.txt.duplicate"
mv "$dir/releases/checksums.txt.duplicate" "$dir/releases/checksums.txt"
if run_installer "$dir" >/dev/null 2>&1; then echo 'duplicate checksum was accepted' >&2; exit 1; fi

current_test=wrong-arch
dir=$(make_case wrong-arch)
# shellcheck disable=SC2016 # Deliberately generate a mock whose variables expand when invoked.
printf '#!/bin/sh\ncase "$1" in -s) echo Linux;; -m) echo sparc;; esac\n' > "$dir/bin/uname"; chmod +x "$dir/bin/uname"
if run_installer "$dir" >/dev/null 2>&1; then echo 'wrong architecture was accepted' >&2; exit 1; fi

current_test=existing-directory
dir=$(make_case existing-directory)
mkdir "$dir/install/daiku"
printf 'untouched\n' > "$dir/install/daiku/sentinel"
if run_installer "$dir" >/dev/null 2>&1; then echo 'existing target directory was accepted' >&2; exit 1; fi
[ "$(cat "$dir/install/daiku/sentinel")" = untouched ]

current_test=existing-symlink
dir=$(make_case existing-symlink)
ln -s /bin/sh "$dir/install/daiku"
if run_installer "$dir" >/dev/null 2>&1; then echo 'existing target symlink was accepted' >&2; exit 1; fi
[ "$(readlink "$dir/install/daiku")" = /bin/sh ]

current_test=existing-special
dir=$(make_case existing-special)
mkfifo "$dir/install/daiku"
if run_installer "$dir" >/dev/null 2>&1; then echo 'existing special target was accepted' >&2; exit 1; fi
[ -p "$dir/install/daiku" ]

current_test=stale-staging
dir=$(make_case stale-staging)
printf 'stale\n' > "$dir/install/.daiku.install.stale"
run_installer "$dir" >/dev/null
[ "$(cat "$dir/install/.daiku.install.stale")" = stale ]

current_test=stale-lock
dir=$(make_case stale-lock)
mkdir "$dir/install/.daiku.install.lock"
printf 'owner\n' > "$dir/install/.daiku.install.lock/sentinel"
lock_error="$dir/lock-error"
if run_installer "$dir" > /dev/null 2> "$lock_error"; then echo 'stale lock was ignored' >&2; exit 1; fi
expected_lock_error="daiku installer: install lock exists at $dir/install/.daiku.install.lock; confirm no daiku installer is active, then remove that directory"
[ "$(cat "$lock_error")" = "$expected_lock_error" ] || { echo 'stale lock error was not actionable' >&2; exit 1; }
[ "$(cat "$dir/install/.daiku.install.lock/sentinel")" = owner ]

current_test=symlink-member
dir=$(make_case symlink-member)
rm "$dir/releases/daiku"
ln -s /bin/sh "$dir/releases/daiku"
tar -czf "$dir/releases/daiku_1.2.3-rc.1_linux_amd64.tar.gz" -C "$dir/releases" daiku
update_checksum "$dir"
if run_installer "$dir" >/dev/null 2>&1; then echo 'symlink archive member was accepted' >&2; exit 1; fi

current_test=traversal-member
dir=$(make_case traversal-member)
printf 'outside\n' > "$dir/outside"
tar -czf "$dir/releases/daiku_1.2.3-rc.1_linux_amd64.tar.gz" -C "$dir/releases" daiku -C "$dir/releases" ../outside 2>/dev/null
update_checksum "$dir"
if run_installer "$dir" >/dev/null 2>&1; then echo 'traversal archive member was accepted' >&2; exit 1; fi

current_test=rollback
dir=$(make_case rollback)
printf '#!/bin/sh\necho old\n' > "$dir/install/daiku"; chmod +x "$dir/install/daiku"
old_digest=$(sha256_file "$dir/install/daiku")
old_mode=$(stat -f '%Lp' "$dir/install/daiku" 2>/dev/null || stat -c '%a' "$dir/install/daiku")
# shellcheck disable=SC2016 # Deliberately generate a mock whose variables expand when invoked.
printf '#!/bin/sh\nsrc= dst=; for arg do src=$dst; dst=$arg; done\ncase "$src:$dst" in */.daiku.install.*:*/daiku) exit 1;; esac\nexec /bin/mv "$@"\n' > "$dir/bin/mv"; chmod +x "$dir/bin/mv"
if run_installer "$dir" >/dev/null 2>&1; then echo 'interrupted install succeeded' >&2; exit 1; fi
[ "$("$dir/install/daiku")" = old ]
[ "$(sha256_file "$dir/install/daiku")" = "$old_digest" ]
[ "$(stat -f '%Lp' "$dir/install/daiku" 2>/dev/null || stat -c '%a' "$dir/install/daiku")" = "$old_mode" ]

current_test=concurrent
dir=$(make_case concurrent)
# shellcheck disable=SC2016 # Deliberately generate a mock whose variables expand when invoked.
printf '%s\n' '#!/bin/sh' \
  'src= dst=; for arg do src=$dst; dst=$arg; done' \
  'case "$src:$dst" in */.daiku.install.*:*/daiku) [ "${MV_INSTALL_DELAY:-0}" = 0 ] || sleep "$MV_INSTALL_DELAY";; esac' \
  'exec /bin/mv "$@"' > "$dir/bin/mv"; chmod +x "$dir/bin/mv"
MV_INSTALL_DELAY=1 run_installer "$dir" >/dev/null & first_pid=$!
attempts=0
while [ ! -d "$dir/install/.daiku.install.lock" ] && [ "$attempts" -lt 100 ]; do attempts=$((attempts + 1)); sleep 0.01; done
if run_installer "$dir" >/dev/null 2>&1; then echo 'concurrent installer entered the critical section' >&2; exit 1; fi
wait "$first_pid"
[ "$("$dir/install/daiku")" = new ]
[ ! -e "$dir/install/.daiku.install.lock" ]

printf 'installer tests passed\n'
