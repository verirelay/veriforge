English | [简体中文](cli.zh-CN.md)

# Command line

```bash
veriforge -init <path> -verirelay-repository <url> -verirelay-commit <sha> -kernel-append <cmdline> [options]
```

## Flags

| Flag | Required | Default | Description |
| --- | --- | --- | --- |
| `-init` | yes | | Path to the Linux amd64 static ELF that runs as micro-system PID 1. |
| `-verirelay-repository` | yes | | Canonical HTTPS repository URL of the VeriRelay source. |
| `-verirelay-commit` | yes | | Lowercase 40-char commit SHA for the VeriRelay binary. |
| `-kernel-append` | yes | | Raw QEMU direct-boot kernel append string, excluding parameters OVMF appends itself such as `initrd=initrd`. |
| `-vcpu-type` | no | `EPYC-v4` | QEMU CPU profile for the Launch Measurement; the Guest Manifest rules allow only `EPYC-v4`. |
| `-out-initrd` | no | `initrd.img` | initrd output path. |
| `-out-vmlinuz` | no | `vmlinuz` | vmlinuz output path. |
| `-out-report` | no | `build-report.json` next to the initrd | Build Report output path. |
| `-cache` | no | `./.veriforge_cache` | Cache directory for the kernel and modules `.deb` downloads. |

`-verirelay-repository`, `-verirelay-commit`, `-vcpu-type` and `-kernel-append` go into the Guest Manifest verbatim. The build fails unless they satisfy the rules in the Protocol's [docs/guest.md](https://github.com/verirelay/protocol/blob/main/docs/guest.md), section "Guest Manifest".

## Source identity

VeriForge takes its own identity from the git checkout of the working directory, so run it from the VeriForge checkout.

The `origin` remote URL and `HEAD` commit go into the Guest Manifest. Uncommitted changes are recorded as `dirty` in `build-report.json`.

`origin` must be a canonical HTTPS repository URL such as `https://github.com/verirelay/veriforge`. A `.git` suffix, an SSH URL, or user info fails the build.

## Canonical build entry point

Build the initrd and vmlinuz with `scripts/forge-initrd.sh`. Local builds, CI, and the Release pipeline all use it, so the Guest Manifest strings are spelled the same everywhere.

```bash
scripts/forge-initrd.sh --init <linux-amd64-static-init> \
                        --verirelay-commit <40-char-lowercase-sha> \
                        --kernel-append <cmdline> [options]
```

| Option | Environment variable | Default |
| --- | --- | --- |
| `--init PATH` | `INIT_BIN` | required |
| `--verirelay-commit SHA` | `VERIRELAY_COMMIT` | required |
| `--kernel-append S` | `KERNEL_APPEND` | required |
| `--verirelay-repository URL` | `VERIRELAY_REPOSITORY` | `https://github.com/verirelay/verirelay` |
| `--vcpu-type T` | `VCPU_TYPE` | `EPYC-v4` |
| `--out-dir DIR` | `OUT_DIR` | `./dist` |
| `--cache DIR` | `CACHE_DIR` | `./.veriforge_cache` |

`VERIFORGE_BIN` points to an already built CLI. The default is `go run` with a pinned `GOTOOLCHAIN`.

An init binary inside the worktree must sit under an ignored path such as `dist/`; otherwise the build records `dirty: true`.
