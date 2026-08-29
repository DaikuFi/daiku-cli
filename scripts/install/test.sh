#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
case_root=$(mktemp -d "${TMPDIR:-/tmp}/daiku-installer-test.XXXXXX")
trap 'rm -rf "$case_root"' EXIT HUP INT TERM

make_case() {
  name=$1
  dir="$case_root/$name"
  mkdir -p "$dir/bin" "$dir/releases" "$dir/install"
  printf '#!/bin/sh\necho new\n' > "$dir/releases/daiku"
  chmod +x "$dir/releases/daiku"
  tar -czf "$dir/releases/daiku_1.2.3-rc.1_linux_amd64.tar.gz" -C "$dir/releases" daiku
  (cd "$dir/releases" && shasum -a 256 daiku_1.2.3-rc.1_linux_amd64.tar.gz > checksums.txt)
  : > "$dir/releases/checksums.txt.sig"; : > "$dir/releases/checksums.txt.pem"
  printf '#!/bin/sh\ncase "$1" in -s) echo Linux;; -m) echo x86_64;; *) exit 2;; esac\n' > "$dir/bin/uname"
  printf '#!/bin/sh\nexit "${COSIGN_EXIT:-0}"\n' > "$dir/bin/cosign"
  printf '#!/bin/sh\nurl=""; out=""; while [ "$#" -gt 0 ]; do case "$1" in --output) out=$2; shift 2;; http*) url=$1; shift;; *) shift;; esac; done; cp "$FIXTURE_DIR/${url##*/}" "$out"\n' > "$dir/bin/curl"
  chmod +x "$dir/bin/"*
  printf '%s\n' "$dir"
}

run_installer() {
  dir=$1
  PATH="$dir/bin:$PATH" FIXTURE_DIR="$dir/releases" DAIKU_INSTALL_DIR="$dir/install" \
    DAIKU_VERSION=v1.2.3-rc.1 "$root/scripts/install/daiku.sh"
}

dir=$(make_case success)
run_installer "$dir" >/dev/null
[ "$("$dir/install/daiku")" = new ]

dir=$(make_case bad-signature)
if COSIGN_EXIT=1 run_installer "$dir" >/dev/null 2>&1; then echo 'invalid signature was accepted' >&2; exit 1; fi
[ ! -e "$dir/install/daiku" ]

dir=$(make_case bad-checksum)
printf '0  daiku_1.2.3-rc.1_linux_amd64.tar.gz\n' > "$dir/releases/checksums.txt"
if run_installer "$dir" >/dev/null 2>&1; then echo 'invalid checksum was accepted' >&2; exit 1; fi

dir=$(make_case wrong-arch)
printf '#!/bin/sh\ncase "$1" in -s) echo Linux;; -m) echo sparc;; esac\n' > "$dir/bin/uname"; chmod +x "$dir/bin/uname"
if run_installer "$dir" >/dev/null 2>&1; then echo 'wrong architecture was accepted' >&2; exit 1; fi

dir=$(make_case rollback)
printf '#!/bin/sh\necho old\n' > "$dir/install/daiku"; chmod +x "$dir/install/daiku"
printf '#!/bin/sh\ncase "$2" in */.daiku.new) exit 1;; esac\nexec /bin/mv "$@"\n' > "$dir/bin/mv"; chmod +x "$dir/bin/mv"
if run_installer "$dir" >/dev/null 2>&1; then echo 'interrupted install succeeded' >&2; exit 1; fi
[ "$("$dir/install/daiku")" = old ]

printf 'installer tests passed\n'
