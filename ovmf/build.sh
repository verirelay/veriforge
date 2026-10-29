#!/usr/bin/env bash
set -euo pipefail

export TZ=UTC
export LC_ALL=C.UTF-8
export LANG=C.UTF-8
export PYTHONHASHSEED=0
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
umask 022

OUT="${1:?output directory required}"
shift
SOURCE_LOCK="/src/source.lock"

lock_value() {
  local key="$1"
  sed -nE "s/^${key} = \"([^\"]+)\"$/\1/p" "$SOURCE_LOCK"
}

EDK2_REPOSITORY="$(lock_value repository)"
EDK2_COMMIT="$(lock_value commit)"

[[ -n "$EDK2_REPOSITORY" ]] || { echo "missing repository in $SOURCE_LOCK" >&2; exit 1; }
[[ "$EDK2_COMMIT" =~ ^[0-9a-f]{40}$ ]] || {
  echo "commit in $SOURCE_LOCK must be a 40-character lowercase SHA-1" >&2
  exit 1
}

git clone "$EDK2_REPOSITORY" edk2
cd edk2
git checkout --detach "$EDK2_COMMIT"
[[ "$(git rev-parse HEAD)" == "$EDK2_COMMIT" ]] || {
  echo "checked-out edk2 revision differs from $SOURCE_LOCK" >&2
  exit 1
}
git submodule update --init --recursive

export SOURCE_DATE_EPOCH
SOURCE_DATE_EPOCH="$(git show -s --format=%ct "$EDK2_COMMIT")"
export WORKSPACE="$PWD"
export PACKAGES_PATH="$PWD"

EXPECTED_PATCH_SHA256="$(sed -nE 's/^sha256 = "([0-9a-f]{64})"$/\1/p' "$SOURCE_LOCK")"
ACTUAL_PATCH_SHA256="$(sha256sum /src/patches/amd-sev-grub.patch | cut -d ' ' -f1)"

[[ -n "$EXPECTED_PATCH_SHA256" ]] || {
  echo "missing patch sha256 in $SOURCE_LOCK" >&2
  exit 1
}

[[ "$ACTUAL_PATCH_SHA256" == "$EXPECTED_PATCH_SHA256" ]] || {
  echo "patch sha256 mismatch: amd-sev-grub.patch" >&2
  echo "expected: $EXPECTED_PATCH_SHA256" >&2
  echo "actual:   $ACTUAL_PATCH_SHA256" >&2
  exit 1
}

git apply --check /src/patches/amd-sev-grub.patch
git apply /src/patches/amd-sev-grub.patch

make -C BaseTools

set +u
source edksetup.sh
set -u

build \
  -a X64 \
  -p OvmfPkg/AmdSev/AmdSevX64.dsc \
  -t GCC \
  -b RELEASE

mkdir -p "$OUT"
cp Build/AmdSev/RELEASE_GCC/FV/OVMF.fd "$OUT/OVMF.fd"

(
  cd "$OUT"
  sha256sum OVMF.fd > OVMF.fd.sha256
)
