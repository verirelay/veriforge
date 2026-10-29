[English](cli.md) | 简体中文

# 命令行

```bash
veriforge -init <path> -verirelay-repository <url> -verirelay-commit <sha> -kernel-append <cmdline> [选项]
```

## Flag

| Flag | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-init` | 是 | | 作为 micro-system PID 1 的 Linux amd64 静态 ELF 路径 |
| `-verirelay-repository` | 是 | | VeriRelay 源码的 canonical HTTPS 仓库 URL |
| `-verirelay-commit` | 是 | | VeriRelay 二进制对应的小写 40 位 commit SHA |
| `-kernel-append` | 是 | | 原始 QEMU direct boot kernel append 字符串，不含 OVMF 自行追加的参数（如 `initrd=initrd`） |
| `-vcpu-type` | 否 | `EPYC-v4` | 启动度量使用的 QEMU CPU profile；来宾启动声明规则只允许 `EPYC-v4` |
| `-out-initrd` | 否 | `initrd.img` | initrd 输出路径 |
| `-out-vmlinuz` | 否 | `vmlinuz` | vmlinuz 输出路径 |
| `-out-report` | 否 | initrd 同目录的 `build-report.json` | 构建报告输出路径 |
| `-cache` | 否 | `./.veriforge_cache` | kernel 与 modules `.deb` 的下载缓存目录 |

`-verirelay-repository`、`-verirelay-commit`、`-vcpu-type` 和 `-kernel-append` 原样写入来宾启动声明。不符合 Protocol [docs/guest.zh-CN.md](https://github.com/verirelay/protocol/blob/main/docs/guest.zh-CN.md) 中「来宾启动声明（Guest Manifest）」一节的规则时，构建失败。

## 源码身份

VeriForge 自身的身份取自工作目录所在的 git checkout，所以要在 VeriForge 的 checkout 中运行。

`origin` remote URL 和 `HEAD` commit 写入来宾启动声明。未提交的改动以 `dirty` 记在 `build-report.json` 中。

`origin` 必须是 canonical HTTPS 仓库 URL，如 `https://github.com/verirelay/veriforge`。带 `.git` 后缀、SSH URL 或 user info 都会使构建失败。

## 统一构建入口

用 `scripts/forge-initrd.sh` 构建 initrd 和 vmlinuz。本地构建、CI 和 Release 流水线都用它，因此各处写出的来宾启动声明字符串完全一致。

```bash
scripts/forge-initrd.sh --init <linux-amd64-static-init> \
                        --verirelay-commit <40位小写sha> \
                        --kernel-append <cmdline> [选项]
```

| 选项 | 环境变量 | 默认值 |
| --- | --- | --- |
| `--init PATH` | `INIT_BIN` | 必填 |
| `--verirelay-commit SHA` | `VERIRELAY_COMMIT` | 必填 |
| `--kernel-append S` | `KERNEL_APPEND` | 必填 |
| `--verirelay-repository URL` | `VERIRELAY_REPOSITORY` | `https://github.com/verirelay/verirelay` |
| `--vcpu-type T` | `VCPU_TYPE` | `EPYC-v4` |
| `--out-dir DIR` | `OUT_DIR` | `./dist` |
| `--cache DIR` | `CACHE_DIR` | `./.veriforge_cache` |

`VERIFORGE_BIN` 指向已构建的 CLI。默认是带固定 `GOTOOLCHAIN` 的 `go run`。

工作树内的 init 二进制必须放在 `dist/` 这类被 Git 忽略的路径下，否则构建会记录 `dirty: true`。
