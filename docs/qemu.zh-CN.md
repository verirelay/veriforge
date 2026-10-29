[English](qemu.md) | 简体中文

# QEMU 验证

## 本地启动（TCG）

`make run-qemu` 调用 `scripts/run-qemu.sh`，在任意宿主机上以 QEMU TCG 启动 `dist/vmlinuz` 和 `dist/initrd.img`。脚本从以下环境变量读取产物路径：

| 环境变量 | 默认值 |
| --- | --- |
| `VMLINUX` | `dist/vmlinuz` |
| `INITRD` | `dist/initrd.img` |

宿主机 8080 端口转发到 Guest 的 8080 端口。`Ctrl+A` 后按 `X` 退出 QEMU。

`examples/mock-init` 是用于本地启动测试的最小 PID 1，由 `make mock-init` 构建，传给 `-init`。

## SEV-SNP 启动

SEV-SNP 启动还需要引导文件：宿主机通过配置盘交付它，并通过 `HOST_DATA` 把它绑定到证明报告。引导文件格式、配置盘布局和绑定方式定义在 Protocol：[docs/guest.zh-CN.md](https://github.com/verirelay/protocol/blob/main/docs/guest.zh-CN.md) 的「引导文件（Bootstrap）」一节。

### 配置盘与 `host-data`

1. 写出配置盘，即引导文件后接零字节 padding，这里补齐到 64 KiB：

   ```bash
   cp bootstrap.json config.img
   truncate -s 64K config.img
   ```

2. 计算 `host-data`，即引导文件字节 SHA-256 的 base64：

   ```bash
   HOST_DATA=$(openssl dgst -sha256 -binary bootstrap.json | base64 -w0)
   ```

两步都使用 VeriProv 所配置的引导文件的原始字节。任何字节差异（包括空白或末尾换行）都会改变 `HOST_DATA`。

`sha256sum bootstrap.json` 以十六进制打印同一 digest，与证明报告中的形式一致。

### QEMU 命令

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

固件、`-cpu`、`-smp`、`host-data` 以外的 `-object sev-snp-guest` 设置以及 `-append` 字符串共同决定启动度量：

- `-cpu` 和 `-append` 与 `/etc/verirelay/manifest.json` 中的 `launch_profile` 一致。
- `-smp` 可以是任意 vCPU 数量，每个数量有各自的启动度量。
- 请使用 `./dist/OVMF.fd`（见 [ovmf/README.zh-CN.md](../ovmf/README.zh-CN.md)），换用其他固件会改变启动度量。

配置盘必须是 CVM 中唯一的 virtio 块设备。

## 期望启动度量

`sev-snp-measure` 按相同的 launch profile 输入和 vCPU 数量离线算出期望的启动度量。命令见 release 仓库 [docs/launch-measurement.zh-CN.md](https://github.com/verirelay/release/blob/main/docs/launch-measurement.zh-CN.md) 的「重新计算」一节。
