#!/bin/sh
# Download this file first, inspect it, then run it. The installer never executes downloaded code.
set -eu

validate_prerelease_version() {
  printf '%s\n' "$1" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-((0|[1-9][0-9]*)|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.((0|[1-9][0-9]*)|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*$'
}

if [ "${1:-}" = --validate-version ]; then
  [ "$#" -eq 2 ] && validate_prerelease_version "$2"
  exit $?
fi

repo=${DAIKU_RELEASE_REPOSITORY:-DaikuFi/daiku-cli}
version=${DAIKU_VERSION:-}
install_dir=${DAIKU_INSTALL_DIR:-"${HOME:?HOME is required}/.local/bin"}
cosign_identity=${DAIKU_COSIGN_IDENTITY:-https://github.com/DaikuFi/daiku-cli/.github/workflows/release.yml@refs/heads/main}
cosign_issuer=${DAIKU_COSIGN_ISSUER:-https://token.actions.githubusercontent.com}

fail() { printf '%s\n' "daiku installer: $*" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || fail 'curl is required'
command -v cosign >/dev/null 2>&1 || fail 'cosign is required to verify the release signature'
command -v tar >/dev/null 2>&1 || fail 'tar is required'

os=$(uname -s)
case "$os" in Darwin) os=darwin ;; Linux) os=linux ;; *) fail "unsupported operating system: $os" ;; esac
arch=$(uname -m)
case "$arch" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) fail "unsupported architecture: $arch" ;; esac

case "$repo" in *[!A-Za-z0-9._/-]*|/*|*..*) fail 'invalid release repository' ;; esac
validate_prerelease_version "$version" || fail 'DAIKU_VERSION must be an exact v-prefixed semantic prerelease'
base="https://github.com/$repo/releases/download/$version"
label=${version#v}
case "$base" in https://*) ;; *) fail 'release URL must use HTTPS' ;; esac

tmp=$(mktemp -d "${TMPDIR:-/tmp}/daiku-install.XXXXXX") || fail 'could not create temporary directory'
backup=
backed_up=false
committed=false
staged=
lock=
target="$install_dir/daiku"
cleanup() {
  if [ -n "$staged" ] && [ -f "$staged" ]; then rm -f "$staged"; fi
  if [ "$committed" != true ] && [ "$backed_up" = true ]; then
    rm -f "$target"
    mv -f "$backup" "$target"
    backed_up=false
  fi
  if [ -n "$backup" ] && [ -f "$backup" ]; then rm -f "$backup"; fi
  if [ -n "$lock" ] && [ -d "$lock" ]; then rmdir "$lock"; fi
  rm -rf "$tmp"
}
trap cleanup EXIT
trap 'exit 130' HUP INT TERM

get() { curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 --output "$2" "$1"; }
get "$base/checksums.txt" "$tmp/checksums.txt"
get "$base/checksums.txt.sig" "$tmp/checksums.txt.sig"
get "$base/checksums.txt.pem" "$tmp/checksums.txt.pem"
cosign verify-blob --certificate "$tmp/checksums.txt.pem" --signature "$tmp/checksums.txt.sig" \
  --certificate-identity "$cosign_identity" --certificate-oidc-issuer "$cosign_issuer" "$tmp/checksums.txt" >/dev/null || fail 'checksum signature verification failed'

archive="daiku_${label}_${os}_${arch}.tar.gz"
get "$base/$archive" "$tmp/$archive"
matches=$(awk -v file="$archive" '$2 == file { count++; value=$1 } END { if (count == 1) print value }' "$tmp/checksums.txt")
printf '%s\n' "$matches" | grep -Eq '^[0-9a-f]{64}$' || fail 'signed checksums must contain exactly one valid artifact digest'
expected=$matches
actual=$(shasum -a 256 "$tmp/$archive" 2>/dev/null || sha256sum "$tmp/$archive")
actual=${actual%% *}
[ "$actual" = "$expected" ] || fail 'artifact checksum verification failed'

mkdir -p "$tmp/unpack" "$install_dir"
members=$(tar -tzf "$tmp/$archive") || fail 'could not inspect archive'
[ "$members" = daiku ] || fail 'archive must contain exactly one member named daiku'
member_type=$(tar -tvzf "$tmp/$archive" | awk 'NR == 1 { print substr($1, 1, 1) } NR > 1 { exit 2 }') || fail 'could not inspect archive member type'
[ "$member_type" = - ] || fail 'daiku archive member must be a regular file'
tar -xzf "$tmp/$archive" -C "$tmp/unpack" daiku
[ -f "$tmp/unpack/daiku" ] || fail 'archive does not contain daiku'

lock_path="$install_dir/.daiku.install.lock"
mkdir "$lock_path" 2>/dev/null || fail 'another install is running or a stale install lock exists'
lock=$lock_path
if [ -L "$target" ] || [ -d "$target" ] || [ -p "$target" ] || [ -b "$target" ] || [ -c "$target" ]; then
  fail 'existing daiku target must be a regular executable file'
fi
if [ -e "$target" ] && { [ ! -f "$target" ] || [ ! -x "$target" ]; }; then
  fail 'existing daiku target must be a regular executable file'
fi

staged=$(mktemp "$install_dir/.daiku.install.XXXXXX") || fail 'could not allocate destination staging file'
cp "$tmp/unpack/daiku" "$staged"
chmod 0755 "$staged"
if [ -e "$target" ]; then
  backup=$(mktemp "$install_dir/.daiku.backup.XXXXXX") || fail 'could not allocate destination backup'
  mv -f "$target" "$backup"
  backed_up=true
fi
mv -f "$staged" "$target"
staged=
committed=true
if [ "$backed_up" = true ]; then rm -f "$backup"; backed_up=false; fi
printf 'Installed daiku %s for %s/%s in %s\n' "$label" "$os" "$arch" "$target"
