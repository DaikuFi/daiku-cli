#!/bin/sh
# Download this file first, inspect it, then run it. The installer never executes downloaded code.
set -eu

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
printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$' || fail 'DAIKU_VERSION must be an exact v-prefixed semantic version'
base="https://github.com/$repo/releases/download/$version"
label=${version#v}
case "$base" in https://*) ;; *) fail 'release URL must use HTTPS' ;; esac

tmp=$(mktemp -d "${TMPDIR:-/tmp}/daiku-install.XXXXXX") || fail 'could not create temporary directory'
backup=
committed=false
staged="$install_dir/.daiku.new"
cleanup() {
  rm -f "$staged"
  if [ "$committed" != true ] && [ -n "$backup" ] && [ -f "$backup" ]; then mv -f "$backup" "$install_dir/daiku"; fi
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
expected=$(awk -v file="$archive" '$2 == file { print $1 }' "$tmp/checksums.txt")
[ -n "$expected" ] || fail 'artifact is absent from signed checksums'
actual=$(shasum -a 256 "$tmp/$archive" 2>/dev/null || sha256sum "$tmp/$archive")
actual=${actual%% *}
[ "$actual" = "$expected" ] || fail 'artifact checksum verification failed'

mkdir -p "$tmp/unpack" "$install_dir"
tar -xzf "$tmp/$archive" -C "$tmp/unpack" daiku
[ -f "$tmp/unpack/daiku" ] || fail 'archive does not contain daiku'
chmod 0755 "$tmp/unpack/daiku"
if [ -e "$install_dir/daiku" ]; then backup="$tmp/daiku.previous"; mv "$install_dir/daiku" "$backup"; fi
mv "$tmp/unpack/daiku" "$staged"
mv -f "$staged" "$install_dir/daiku"
committed=true
printf 'Installed daiku %s for %s/%s in %s\n' "$label" "$os" "$arch" "$install_dir/daiku"
