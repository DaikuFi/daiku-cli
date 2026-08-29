#!/bin/sh
set -eu

[ "$#" -eq 3 ] || { echo 'usage: homebrew.sh VERSION CHECKSUMS OUTPUT' >&2; exit 2; }
version=${1#v}
checksums=$2
output=$3
repo=${DAIKU_RELEASE_REPOSITORY:-DaikuFi/daiku-cli}
base="https://github.com/$repo/releases/download/v$version"

checksum() {
  value=$(awk -v file="daiku_${version}_$1_$2.tar.gz" '$2 == file { print $1 }' "$checksums")
  [ -n "$value" ] || { echo "missing checksum for $1/$2" >&2; exit 1; }
  printf '%s' "$value"
}

darwin_amd64=$(checksum darwin amd64)
darwin_arm64=$(checksum darwin arm64)
linux_amd64=$(checksum linux amd64)
linux_arm64=$(checksum linux arm64)

sed \
  -e "s|{{ .Version }}|$version|g" \
  -e "s|{{ .DarwinArm64URL }}|$base/daiku_${version}_darwin_arm64.tar.gz|g" \
  -e "s|{{ .DarwinArm64SHA256 }}|$darwin_arm64|g" \
  -e "s|{{ .DarwinAmd64URL }}|$base/daiku_${version}_darwin_amd64.tar.gz|g" \
  -e "s|{{ .DarwinAmd64SHA256 }}|$darwin_amd64|g" \
  -e "s|{{ .LinuxArm64URL }}|$base/daiku_${version}_linux_arm64.tar.gz|g" \
  -e "s|{{ .LinuxArm64SHA256 }}|$linux_arm64|g" \
  -e "s|{{ .LinuxAmd64URL }}|$base/daiku_${version}_linux_amd64.tar.gz|g" \
  -e "s|{{ .LinuxAmd64SHA256 }}|$linux_amd64|g" \
  packaging/homebrew/daiku.rb.tmpl > "$output"
