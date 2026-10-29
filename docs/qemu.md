English | [简体中文](qemu.zh-CN.md)

# QEMU verification

## Local boot (TCG)

`make run-qemu` calls `scripts/run-qemu.sh`, which boots `dist/vmlinuz` and `dist/initrd.img` under QEMU TCG on any host. The script takes the artifact paths from these environment variables:

| Environment variable | Default |
| --- | --- |
| `VMLINUX` | `dist/vmlinuz` |
| `INITRD` | `dist/initrd.img` |

Host port 8080 forwards to Guest port 8080. Exit QEMU with `Ctrl+A`, then `X`.

`examples/mock-init` is a minimal PID 1 for local boot tests, built by `make mock-init` and passed to `-init`.

## SEV-SNP launch

An SEV-SNP launch also needs a Bootstrap: the host delivers it on the Config Disk and binds it to the Attestation Report through `HOST_DATA`. The Bootstrap format, the Config Disk layout, and the binding are specified in the Protocol: [docs/guest.md](https://github.com/verirelay/protocol/blob/main/docs/guest.md), section "Bootstrap".

### Config Disk and `host-data`

1. Write the Config Disk, the Bootstrap file followed by zero padding, here to 64 KiB:

   ```bash
   cp bootstrap.json config.img
   truncate -s 64K config.img
   ```

2. Compute `host-data`, the base64 of the SHA-256 of the Bootstrap file bytes:

   ```bash
   HOST_DATA=$(openssl dgst -sha256 -binary bootstrap.json | base64 -w0)
   ```

Both steps use the exact bytes of the Bootstrap file VeriProv is configured with. Any byte difference, including whitespace or a trailing newline, changes `HOST_DATA`.

`sha256sum bootstrap.json` prints the same digest in hex, as it appears in the Attestation Report.

### QEMU command

```bash
sudo qemu-system-x86_64 \
  -enable-kvm \
  -cpu EPYC-v4 \
  -smp <vcpus> \
  -m 512M \
  -machine q35,confidential-guest-support=sev0,memory-encryption=sev0,memory-backend=ram0,vmport=off \
  -object memory-backend-memfd,id=ram0,size=512M,share=on \
  -object sev-snp-guest,id=sev0,cbitpos=51,reduced-phys-bits=1,kernel-hashes=on,policy=0x30000,host-data="$HOST_DATA" \
  -bios ./dist/OVMF.fd \
  -kernel ./dist/vmlinuz \
  -initrd ./dist/initrd.img \
  -append "<kernel_append>" \
  -drive file=config.img,if=virtio,format=raw,readonly=on \
  -nographic \
  -netdev user,id=net0,hostfwd=tcp::8080-:8080 \
  -device virtio-net-pci,netdev=net0
```

Together with the firmware, `-cpu`, `-smp`, the `-object sev-snp-guest` settings other than `host-data`, and the `-append` string determine the Launch Measurement:

- `-cpu` and `-append` match the `launch_profile` recorded in `/etc/verirelay/manifest.json`.
- `-smp` is any vCPU count; each count has its own Launch Measurement.
- Use `./dist/OVMF.fd` (see [ovmf/README.md](../ovmf/README.md)); any other firmware changes the Launch Measurement.

The Config Disk must be the CVM's only virtio block device.

## Expected Launch Measurement

`sev-snp-measure` recomputes the expected Launch Measurement offline from the same launch profile inputs and vCPU count. The command is in the release repository's [docs/launch-measurement.md](https://github.com/verirelay/release/blob/main/docs/launch-measurement.md), section Recomputing.
