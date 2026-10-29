[English](development.md) | 简体中文

# 开发

## 依赖

- Go，版本以 `go.mod` 为准。
- 构建 initrd 需要 `dpkg-deb`、`zstd`、`xz`。
- 启动测试需要 `qemu-system-x86_64`。
- `make lint` 需要 golangci-lint v2，版本固定在 `.github/workflows/ci.yml`。
- OVMF 构建需要带 buildx 的 Docker。

## Make target

| Target | 作用 |
| --- | --- |
| `make build` | 构建 CLI 到 `dist/veriforge` |
| `make mock-init` | 构建静态 mock init 到 `dist/mock-init` |
| `make forge` | 构建 CLI 和 mock init，产出 `dist/initrd.img`、`dist/vmlinuz`、`dist/build-report.json` |
| `make run-qemu` | 通过 `scripts/run-qemu.sh` 在 QEMU TCG 中启动产物 |
| `make test` | `go test ./...` |
| `make lint` | `golangci-lint run` |
| `make check` | test + build + mock-init + lint |
| `make clean` | 删除 `dist/` |

`make forge` 需要 `VERIRELAY_REPOSITORY`、`VERIRELAY_COMMIT`、`KERNEL_APPEND`。

## 测试

## CI

### `ci.yml`

push 到 `main` 和 pull request 时执行：

- 格式
- vet
- `golangci-lint`
- 测试
- CLI 和 mock-init 构建
- 构建后工作树干净断言
- `actionlint`

手动输入 `full_forge` 追加一次用 mock init 的完整 initrd 构建。其 `smoke-` artifact 仅用于测试。

### `ovmf.yml`

`ovmf/` 变化时，构建两次 `OVMF.fd`，第二次使用 `--no-cache`。两次构建都要与 `artifacts.lock` 一致且彼此一致，否则失败。

### `release.yml`

`v*` tag 时，产出面向直接使用 VeriForge 的用户的组件级发布：

- CLI 二进制
- `OVMF.fd`
- checksum
- `artifacts.lock`
- 组件构建报告（`veriforge-component-build.json`）
- cosign bundle
- 每个资产的 GitHub artifact attestation

同样执行 OVMF 双构建。

用 GitHub CLI 验证组件发布资产的 GitHub artifact attestation，例如：

```bash
gh attestation verify OVMF.fd \
  --repo verirelay/veriforge \
  --signer-workflow verirelay/veriforge/.github/workflows/release.yml
```

产品级发布在 [release](https://github.com/verirelay/release) 仓库。它按 `versions.lock` 固定的 commit 重建全部产物，产出发布证据链（`chain.json`）。
