English | [简体中文](manifest.zh-CN.md)

# Guest Manifest and Build Report

The Guest Manifest schema is defined in the Protocol: [docs/guest.md](https://github.com/verirelay/protocol/blob/main/docs/guest.md).

## `/etc/verirelay/manifest.json`

The Guest Manifest, `verirelay-guest-manifest/1`.

| Field | Source |
| --- | --- |
| `source.verirelay` | Declared identity of the `-init` binary: the `-verirelay-repository` and `-verirelay-commit` values, verbatim. |
| `source.veriforge` | VeriForge's own identity from the build-time git checkout (`origin` URL and `HEAD`). |
| `launch_profile` | Inputs for the Launch Measurement: `vcpu_type` and the raw `kernel_append` string from `-vcpu-type` and `-kernel-append`. |

The other `launch_profile` fields are Protocol constants.

## `build-report.json`

The Build Report, `verirelay-build-report/1`, written next to `initrd.img` (or to `-out-report`).

| Field | Contents |
| --- | --- |
| `version` | `verirelay-build-report/1`. |
| `manifest` | The same Guest Manifest object that sits inside the initrd. |
| `verirelay_binary` | SHA-256 and size of the `-init` binary. |
| `veriforge` | `repository`, `commit`, and `dirty` of the VeriForge checkout. |
| `target` | `os`, `arch`, and `kernel_release` of the produced artifacts. |
| `toolchain` | `go_version`: the version of the Go toolchain that ran the build, such as `go1.26.10`. |
| `kernel` | `kernel_url`, `kernel_sha256`, `modules_url`, `modules_sha256` of the pinned inputs. |
| `source_date_epoch` | The epoch value used for deterministic output. |
| `cpio` | `format` (`newc`) and the pinned `uid`, `gid`, `mtime` of entries. |
| `compression` | `algorithm` (`gzip`) and `level`. |
| `artifacts` | SHA-256 and size of `manifest`, `binary`, `kernel`, and `initrd`. |
