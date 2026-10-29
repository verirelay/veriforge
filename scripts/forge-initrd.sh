#!/usr/bin/env bash
#
# The single entry point for building the initrd and vmlinuz. Local builds, CI,
# and the Release pipeline must all call this script: these parameters are
# written verbatim into /etc/verirelay/manifest.json inside the initrd, so any
# difference in spelling produces a different digest.
#
# Usage:
#   scripts/forge-initrd.sh --init <linux-amd64-static-init> \
#                           --verirelay-commit <40-char-lowercase-sha> \
#                           --kernel-append <cmdline> [options]
#
# Options (environment variables also accepted):
#   --init PATH                     INIT_BIN                  required
#   --verirelay-commit SHA          VERIRELAY_COMMIT          required
#   --kernel-append S               KERNEL_APPEND             required
#   --verirelay-repository URL      VERIRELAY_REPOSITORY      default https://github.com/verirelay/verirelay
#   --vcpu-type T                   VCPU_TYPE                 default EPYC-v4
#   --out-dir DIR                   OUT_DIR                   default ./dist
#   --cache DIR                     CACHE_DIR                 default ./.veriforge_cache
#
# Hard rules:
#   1. --init must be a statically linked x86-64 ELF (veriforge itself rejects
#      dynamically linked binaries).
#   2. An init binary inside this working tree must sit in a .gitignore'd path
#      such as dist/, otherwise build-report.json records dirty=true.
#   3. --verirelay-repository and this repository's origin remote must be
#      canonical HTTPS URLs without .git, such as
#      https://github.com/verirelay/veriforge; veriforge rejects other spellings.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"

INIT_BIN="${INIT_BIN:-}"
VERIRELAY_COMMIT="${VERIRELAY_COMMIT:-}"
KERNEL_APPEND="${KERNEL_APPEND:-}"
VERIRELAY_REPOSITORY="${VERIRELAY_REPOSITORY:-https://github.com/verirelay/verirelay}"
VCPU_TYPE="${VCPU_TYPE:-EPYC-v4}"
OUT_DIR="${OUT_DIR:-$PROJECT_DIR/dist}"
CACHE_DIR="${CACHE_DIR:-$PROJECT_DIR/.veriforge_cache}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --init) INIT_BIN="$2"; shift 2 ;;
    --verirelay-commit) VERIRELAY_COMMIT="$2"; shift 2 ;;
    --kernel-append) KERNEL_APPEND="$2"; shift 2 ;;
    --verirelay-repository) VERIRELAY_REPOSITORY="$2"; shift 2 ;;
    --vcpu-type) VCPU_TYPE="$2"; shift 2 ;;
    --out-dir) OUT_DIR="$2"; shift 2 ;;
    --cache) CACHE_DIR="$2"; shift 2 ;;
    -h|--help) sed -nE '3,/^set -euo/ s/^# ?//p' "${BASH_SOURCE[0]}"; exit 0 ;;
    *) echo "error: unknown option $1" >&2; exit 2 ;;
  esac
done

die() { echo "error: $*" >&2; exit 1; }

[[ -n "$INIT_BIN" ]] || die "--init is required (or set INIT_BIN)"
[[ -n "$VERIRELAY_COMMIT" ]] || die "--verirelay-commit is required (or set VERIRELAY_COMMIT)"
[[ -n "$KERNEL_APPEND" ]] || die "--kernel-append is required (or set KERNEL_APPEND)"

INIT_ABS="$(cd -- "$(dirname -- "$INIT_BIN")" && pwd)/$(basename -- "$INIT_BIN")"
[[ -f "$INIT_ABS" ]] || die "init binary does not exist: $INIT_ABS"

mkdir -p "$OUT_DIR" "$CACHE_DIR"

OUT_INITRD="$OUT_DIR/initrd.img"
OUT_VMLINUZ="$OUT_DIR/vmlinuz"
OUT_REPORT="$OUT_DIR/build-report.json"

cd "$PROJECT_DIR"

# VERIFORGE_BIN reuses an already built CLI; the default is go run. The
# toolchain must be pinned: toolchain.go_version in build-report.json comes
# from runtime.Version(), so identical inputs report differently across hosts
# with different Go versions.
if [[ -n "${VERIFORGE_BIN:-}" ]]; then
  [[ -x "$VERIFORGE_BIN" ]] || die "VERIFORGE_BIN is not executable: $VERIFORGE_BIN"
  RUN=("$VERIFORGE_BIN")
else
  RUN=(env GOENV=off GOFLAGS='' GOEXPERIMENT='' GOTOOLCHAIN=go1.26.10 go run ./cmd/veriforge)
fi

"${RUN[@]}" \
  -init "$INIT_ABS" \
  -out-initrd "$OUT_INITRD" \
  -out-vmlinuz "$OUT_VMLINUZ" \
  -out-report "$OUT_REPORT" \
  -cache "$CACHE_DIR" \
  -verirelay-commit "$VERIRELAY_COMMIT" \
  -verirelay-repository "$VERIRELAY_REPOSITORY" \
  -vcpu-type "$VCPU_TYPE" \
  -kernel-append "$KERNEL_APPEND"

echo "------------------------------------------------------------------------"
echo "initrd:     $OUT_INITRD"
echo "vmlinuz:    $OUT_VMLINUZ"
echo "report:     $OUT_REPORT"
if command -v jq >/dev/null 2>&1; then
  jq -r '"initrd_sha256        = \"\(.artifacts.initrd.sha256)\"\nvmlinuz_sha256       = \"\(.artifacts.kernel.sha256)\"\nverirelay_bin_sha256 = \"\(.artifacts.binary.sha256)\"\ndirty                = \(.veriforge.dirty)"' "$OUT_REPORT"
  if ! jq -e '.veriforge.dirty == false' "$OUT_REPORT" >/dev/null; then
    # List the concrete entries instead of only reporting dirty=true. Common
    # sources: uncommitted docs, unignored build outputs. dirty does not enter
    # the initrd, but it fails the release pipeline's publish assertion.
    echo "warning: build-report.json records dirty=true; the following changes are uncommitted:" >&2
    git -C "$PROJECT_DIR" status --porcelain | sed 's/^/         /' >&2
    echo "         Commit them, or add output-like files to .gitignore." >&2
  fi
fi
