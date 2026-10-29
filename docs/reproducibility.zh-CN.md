[English](reproducibility.md) | 简体中文

# 可复现性

VeriForge 的构建是确定性的：相同的固定输入在相同 Go 工具链下产出逐字节一致的产物。

## 固定输入

- **内核**：kernel 和 modules `.deb` 的 URL 与 SHA-256、`kernel_release` 以及必需的模块都在 `internal/config/defaults.go` 中。
- **内核模块**：initrd 带上 `ccp`、`tsm_report`、`sev-guest`、`virtio_blk`、`virtio_net` 及其依赖，内核已内建的除外。内核以这两种方式都提供不了其中某个模块时，构建失败。
- **CA bundle**：内置在 `internal/forge/cacerts/ca-certificates.crt`，按 `ca-certificates.crt.sha256` 校验。
- **来宾启动声明中的值**：`-verirelay-repository`、`-verirelay-commit`、`-vcpu-type`、`-kernel-append` 的值以及 VeriForge 的 `origin` 与 `HEAD` 原样记录，写法本身就是 initrd 字节的一部分。

## 确定性归档

initrd 是 SVR4 `newc` 格式的 CPIO 归档，以 `default` level 的 gzip 压缩。每个条目的 UID、GID、mtime 都是 0。

## 可校验的缓存

每次构建都按固定的 SHA-256 校验 `-cache` 中的 `.deb` 包，不匹配的重新下载。下载结果仍不匹配则构建失败。

## 固定工具链

`build-report.json` 的 `toolchain.go_version` 是执行构建的 Go 工具链。未设置 `VERIFORGE_BIN` 时，`scripts/forge-initrd.sh` 通过 `GOTOOLCHAIN` 固定它，并清空 `GOENV`、`GOFLAGS`、`GOEXPERIMENT`。

## 干净工作树

`git status --porcelain` 报告任何未提交的改动（tracked 或 untracked）时，`build-report.json` 的 `veriforge.dirty` 为 `true`。`.gitignore` 覆盖的文件（如 `dist/`）不计入。

publish 模式的发布构建要求 `dirty == false`。
