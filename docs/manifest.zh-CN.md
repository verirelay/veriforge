[English](manifest.md) | 简体中文

# 来宾启动声明与构建报告

来宾启动声明的 schema 定义在 Protocol：[docs/guest.zh-CN.md](https://github.com/verirelay/protocol/blob/main/docs/guest.zh-CN.md)。

## `/etc/verirelay/manifest.json`

来宾启动声明，`verirelay-guest-manifest/1`。

| 字段 | 来源 |
| --- | --- |
| `source.verirelay` | `-init` 二进制的声明身份：`-verirelay-repository` 与 `-verirelay-commit` 的值，原样记录 |
| `source.veriforge` | VeriForge 自身身份，取自构建时的 git checkout（`origin` URL 和 `HEAD`） |
| `launch_profile` | 启动度量的输入：`vcpu_type` 和原始 `kernel_append` 字符串，分别取自 `-vcpu-type`、`-kernel-append` |

`launch_profile` 的其余字段是 Protocol 常量。

## `build-report.json`

构建报告，`verirelay-build-report/1`，写在 `initrd.img` 旁（或 `-out-report` 指定处）。

| 字段 | 内容 |
| --- | --- |
| `version` | `verirelay-build-report/1` |
| `manifest` | 与 initrd 内一致的来宾启动声明对象 |
| `verirelay_binary` | `-init` 二进制的 SHA-256 与大小 |
| `veriforge` | VeriForge checkout 的 `repository`、`commit`、`dirty` |
| `target` | 产物的 `os`、`arch`、`kernel_release` |
| `toolchain` | `go_version`：执行构建的 Go 工具链版本，如 `go1.26.10` |
| `kernel` | 固定输入的 `kernel_url`、`kernel_sha256`、`modules_url`、`modules_sha256` |
| `source_date_epoch` | 确定性输出使用的 epoch 值 |
| `cpio` | `format`（`newc`）以及条目固定的 `uid`、`gid`、`mtime` |
| `compression` | `algorithm`（`gzip`）与 `level` |
| `artifacts` | `manifest`、`binary`、`kernel`、`initrd` 的 SHA-256 与大小 |
