English | [简体中文](reproducibility.zh-CN.md)

# Reproducibility

VeriForge builds are deterministic: the same pinned inputs on the same Go toolchain produce byte-identical artifacts.

## Pinned inputs

- **Kernel**: the kernel and modules `.deb` URLs and SHA-256 hashes, the `kernel_release`, and the required modules live in `internal/config/defaults.go`.
- **Kernel modules**: the initrd ships `ccp`, `tsm_report`, `sev-guest`, `virtio_blk`, and `virtio_net` with their dependencies, unless the kernel builds them in. A kernel that provides one of them neither way fails the build.
- **CA bundle**: vendored at `internal/forge/cacerts/ca-certificates.crt` and checked against `ca-certificates.crt.sha256`.
- **Guest Manifest values**: the `-verirelay-repository`, `-verirelay-commit`, `-vcpu-type`, and `-kernel-append` values and the VeriForge `origin` and `HEAD` are recorded verbatim, so their exact spelling is part of the initrd bytes.

## Deterministic archive

The initrd is an SVR4 `newc` CPIO archive, gzip-compressed at the `default` level. Every entry has UID 0, GID 0, and mtime 0.

## Verified cache

Every build checks the `.deb` packages in `-cache` against the pinned SHA-256 and downloads any that do not match. A download that does not match fails the build.

## Pinned toolchain

`toolchain.go_version` in `build-report.json` is the Go toolchain that ran the build. Unless `VERIFORGE_BIN` is set, `scripts/forge-initrd.sh` pins it through `GOTOOLCHAIN` and clears `GOENV`, `GOFLAGS`, and `GOEXPERIMENT`.

## Clean worktree

`veriforge.dirty` in `build-report.json` is `true` when `git status --porcelain` reports any uncommitted change, tracked or untracked. Files covered by `.gitignore`, such as `dist/`, do not count.

Release builds in publish mode require `dirty == false`.
