#!/bin/sh
set -eu

dist=${1:?artifact directory is required}
version=${2:?artifact version is required}

printf '%s\n' "$version" | grep -Eq '^[0-9A-Za-z.-]+$' || {
  echo 'artifact-check: invalid snapshot version metadata' >&2
  exit 1
}

expected=$(printf '%s\n' \
  "daiku_${version}_darwin_amd64.tar.gz" \
  "daiku_${version}_darwin_arm64.tar.gz" \
  "daiku_${version}_linux_amd64.tar.gz" \
  "daiku_${version}_linux_arm64.tar.gz" | sort)
actual=$(find "$dist" -maxdepth 1 -type f -name '*.tar.gz' -exec basename {} \; | sort)
if [ "$actual" != "$expected" ]; then
  echo 'artifact-check: release archive set does not match supported targets' >&2
  printf '%s\n' 'expected:' "$expected" 'actual:' "$actual" >&2
  exit 1
fi

test -f "$dist/checksums.txt" || {
  echo 'artifact-check: checksums.txt is missing' >&2
  exit 1
}
checksum_names=$(awk '
  NF != 2 || $1 !~ /^[0-9a-f]{64}$/ { invalid = 1; next }
  { print $2 }
  END { if (invalid) exit 1 }
' "$dist/checksums.txt" | sort) || {
  echo 'artifact-check: checksums.txt has an invalid entry' >&2
  exit 1
}
if [ "$checksum_names" != "$expected" ]; then
  echo 'artifact-check: checksum filename set does not match supported targets' >&2
  printf '%s\n' 'expected:' "$expected" 'actual:' "$checksum_names" >&2
  exit 1
fi

for archive in $expected; do
  archive="$dist/$archive"
  test "$(tar -tzf "$archive")" = daiku || {
    echo "artifact-check: $archive must contain only the daiku binary" >&2
    exit 1
  }
done

(cd "$dist" && shasum -a 256 -c checksums.txt)
