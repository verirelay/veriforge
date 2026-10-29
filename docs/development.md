English | [简体中文](development.zh-CN.md)

# Development

## Prerequisites

- Go, at the version in `go.mod`.
- `dpkg-deb`, `zstd`, `xz` for the initrd build.
- `qemu-system-x86_64` for the boot test.
- `golangci-lint` v2 for `make lint`, at the version pinned in `.github/workflows/ci.yml`.
- Docker with buildx for the OVMF build.

## Make targets

| Target | What it does |
| --- | --- |
| `make build` | Build the CLI to `dist/veriforge`. |
| `make mock-init` | Build the static mock init to `dist/mock-init`. |
| `make forge` | Build the CLI and mock init and produce `dist/initrd.img`, `dist/vmlinuz`, `dist/build-report.json`. |
| `make run-qemu` | Boot the artifacts in QEMU TCG via `scripts/run-qemu.sh`. |
| `make test` | `go test ./...` |
| `make lint` | `golangci-lint run` |
| `make check` | test + build + mock-init + lint. |
| `make clean` | Remove `dist/`. |

`make forge` requires `VERIRELAY_REPOSITORY`, `VERIRELAY_COMMIT`, and `KERNEL_APPEND`.

## Tests

## CI

### `ci.yml`

On push to `main` and on pull requests, it runs:

- formatting;
- vet;
- `golangci-lint`;
- tests;
- CLI and mock-init builds;
- a clean-worktree assertion after the build;
- `actionlint`.

The manual input `full_forge` adds a full initrd build with mock init. Its `smoke-` artifacts are for testing only.

### `ovmf.yml`

On changes under `ovmf/`, it builds `OVMF.fd` twice, the second time with `--no-cache`, and fails unless both builds match `artifacts.lock` and each other.

### `release.yml`

On tags `v*`, it produces the component release for direct users of VeriForge:

- the CLI binary;
- `OVMF.fd`;
- checksums;
- `artifacts.lock`;
- a component build report (`veriforge-component-build.json`);
- cosign bundles;
- a GitHub artifact attestation for every asset.

It runs the same OVMF double build.

Verify a component release asset's GitHub artifact attestation with the GitHub CLI, for example:

```bash
gh attestation verify OVMF.fd \
  --repo verirelay/veriforge \
  --signer-workflow verirelay/veriforge/.github/workflows/release.yml
```

The product-level release lives in the [release](https://github.com/verirelay/release) repository. It rebuilds every artifact from the commits pinned in `versions.lock` and publishes the Release Evidence Chain (`chain.json`).
