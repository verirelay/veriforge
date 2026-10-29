[English](README.md) | 简体中文

# 可复现 OVMF 构建

本目录构建用于 AMD SEV/SNP 的 OVMF 固件。

## 构建输入

| 文件 | 职责 |
| --- | --- |
| `Dockerfile` | 固定 Ubuntu 基础镜像 digest、APT snapshot 和顶层构建依赖版本，执行构建 |
| `artifacts.lock` | 完整 dpkg 包闭包的审计基线，构建环境与它不一致时构建失败 |
| `source.lock` | 锁定 edk2 仓库、commit 和本地 patch 的 SHA-256 |
| `patches/amd-sev-grub.patch` | 针对锁定 edk2 版本的 GRUB 兼容性 patch |
| `build.sh` | 校验锁定的源码与 patch，执行 OVMF 构建 |

期望固件 digest 只固定在 release 仓库 `versions.lock` 的 `[expected].ovmf_sha256` 中。

## 构建

从仓库根目录执行：

1. 构建固件：

   ```bash
   mkdir -p dist

   docker buildx build \
     --platform linux/amd64 \
     --target export \
     --output type=local,dest=dist \
     -f ovmf/Dockerfile .
   ```

2. 验证导出的固件：

   ```bash
   (cd dist && sha256sum -c OVMF.fd.sha256)
   ```

3. 验证实际 apt/dpkg 环境与仓库基线一致：

   ```bash
   diff -u ovmf/artifacts.lock dist/artifacts.lock
   ```

## 双构建可复现性

```bash
mkdir -p dist/ovmf-repro/one dist/ovmf-repro/two

docker buildx build \
  --platform linux/amd64 \
  --target export \
  --output type=local,dest=dist/ovmf-repro/one \
  -f ovmf/Dockerfile .

docker buildx build \
  --platform linux/amd64 \
  --no-cache \
  --target export \
  --output type=local,dest=dist/ovmf-repro/two \
  -f ovmf/Dockerfile .

diff -u ovmf/artifacts.lock dist/ovmf-repro/one/artifacts.lock
diff -u dist/ovmf-repro/one/artifacts.lock dist/ovmf-repro/two/artifacts.lock

sha256sum \
  dist/ovmf-repro/one/OVMF.fd \
  dist/ovmf-repro/two/OVMF.fd

cmp -s \
  dist/ovmf-repro/one/OVMF.fd \
  dist/ovmf-repro/two/OVMF.fd \
  && echo reproducible \
  || echo different
```

预期：两个 `diff` 命令输出为空，最后一步打印：

```text
reproducible
```

## 更新 `artifacts.lock`

`artifacts.lock` 随以下任一输入一起变化：

- Ubuntu 基础镜像 digest
- APT snapshot 时间点
- GCC、GRUB、Python、NASM、IASL 等工具链包
- Dockerfile 中的顶层 APT 包或版本
- edk2 新版本引入的系统依赖变化

更新流程：

1. 修改 `Dockerfile` 中要更新的输入。
2. 临时移除 Dockerfile 中的运行时环境检查：

   ```dockerfile
   diff -u /src/artifacts.lock /tmp/artifacts.lock && \
   ```

   不要提交这处移除。
3. 不使用缓存构建候选环境：

   ```bash
   mkdir -p dist/ovmf-candidate

   docker buildx build \
     --platform linux/amd64 \
     --no-cache \
     --target export \
     --output type=local,dest=dist/ovmf-candidate \
     -f ovmf/Dockerfile .
   ```

4. 评审候选依赖差异：

   ```bash
   diff -u ovmf/artifacts.lock dist/ovmf-candidate/artifacts.lock
   ```

   重点检查 `gcc`、`g++`、`binutils`、`grub*`、`python*`、`nasm`、`acpica-tools`、`openssl` 和 `libc6`。
5. 把审核通过的差异写入基线：

   ```bash
   cp dist/ovmf-candidate/artifacts.lock ovmf/artifacts.lock
   ```

6. 恢复 Dockerfile 中的 `diff -u /src/artifacts.lock /tmp/artifacts.lock` 运行时检查。
7. 执行上面的双构建验证，得到 `reproducible` 后再提交 Dockerfile 与新的 `artifacts.lock`。

## 更新 edk2 或 patch

1. 更新 edk2 时，把 `source.lock` 的 `commit` 改为新的 edk2 commit，并保持 patch 可应用。
2. 修改 patch 时，把它的新哈希写入 `source.lock` 中对应 `[[patches]]` 条目的 `sha256`：

   ```bash
   sha256sum ovmf/patches/amd-sev-grub.patch
   ```

3. 执行上面的双构建可复现验证。
