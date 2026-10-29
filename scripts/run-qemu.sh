#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"

VMLINUX="${VMLINUX:-$PROJECT_DIR/dist/vmlinuz}"
INITRD="${INITRD:-$PROJECT_DIR/dist/initrd.img}"

if ! command -v qemu-system-x86_64 >/dev/null 2>&1; then
  echo "error: qemu-system-x86_64 not found. Please install QEMU first." >&2
  exit 1
fi

if [[ ! -f "$VMLINUX" || ! -f "$INITRD" ]]; then
  echo "error: kernel or initrd not found. Run 'make forge' first." >&2
  echo "VMLINUX=$VMLINUX" >&2
  echo "INITRD=$INITRD" >&2
  exit 1
fi

echo "kernel: $VMLINUX"
echo "initrd: $INITRD"
echo "starting QEMU TCG VM; host port 8080 forwards to guest port 8080"
echo "press Ctrl+A, then X to exit"
echo "------------------------------------------------------------------------"

qemu-system-x86_64 \
  -machine q35,accel=tcg,vmport=off \
  -cpu max \
  -m 512M \
  -kernel "$VMLINUX" \
  -initrd "$INITRD" \
  -append "console=ttyS0 quiet panic=1" \
  -nographic \
  -netdev user,id=net0,hostfwd=tcp::8080-:8080 \
  -device virtio-net-pci,netdev=net0
