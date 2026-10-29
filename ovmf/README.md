English | [简体中文](README.zh-CN.md)

# Reproducible OVMF build

This directory builds the OVMF firmware used for AMD SEV/SNP.

## Build inputs

| File | Role |
| --- | --- |
| `Dockerfile` | Pins the Ubuntu base image digest, the APT snapshot, and the top-level build dependency versions; runs the build. |
| `artifacts.lock` | Audited baseline of the complete dpkg package closure; the build fails if the build environment differs. |
| `source.lock` | Locks the edk2 repository, commit, and the SHA-256 of each local patch. |
| `patches/amd-sev-grub.patch` | GRUB compatibility patch applied to the locked edk2 version. |
| `build.sh` | Verifies the locked source and patches and runs the OVMF build. |

The expected firmware digest is pinned only in the release repository's `versions.lock`, under `[expected].ovmf_sha256`.

## Build

Run from the repository root:

1. Build the firmware:

   ```bash
   mkdir -p dist

   docker buildx build \
     --platform linux/amd64 \
     --target export \
     --output type=local,dest=dist \
     -f ovmf/Dockerfile .
   ```

2. Verify the exported firmware:

   ```bash
   (cd dist && sha256sum -c OVMF.fd.sha256)
   ```

3. Verify that the actual apt/dpkg environment matches the repository baseline:

   ```bash
   diff -u ovmf/artifacts.lock dist/artifacts.lock
   ```

## Double-build reproducibility

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

Expected: both `diff` commands print nothing, and the last step prints:

```text
reproducible
```

## Updating `artifacts.lock`

`artifacts.lock` changes together with one of these inputs:

- the Ubuntu base image digest;
- the APT snapshot timestamp;
- the toolchain packages such as GCC, GRUB, Python, NASM, or IASL;
- a top-level APT package or version in the Dockerfile;
- system dependencies introduced by a new edk2 version.

Update procedure:

1. Edit the input in `Dockerfile`.
2. Temporarily remove the runtime environment check in the Dockerfile:

   ```dockerfile
   diff -u /src/artifacts.lock /tmp/artifacts.lock && \
   ```

   Do not commit this removal.
3. Build the candidate environment without cache:

   ```bash
   mkdir -p dist/ovmf-candidate

   docker buildx build \
     --platform linux/amd64 \
     --no-cache \
     --target export \
     --output type=local,dest=dist/ovmf-candidate \
     -f ovmf/Dockerfile .
   ```

4. Review the candidate dependency diff:

   ```bash
   diff -u ovmf/artifacts.lock dist/ovmf-candidate/artifacts.lock
   ```

   Check `gcc`, `g++`, `binutils`, `grub*`, `python*`, `nasm`, `acpica-tools`, `openssl`, and `libc6` in particular.
5. Accept the diff into the baseline:

   ```bash
   cp dist/ovmf-candidate/artifacts.lock ovmf/artifacts.lock
   ```

6. Restore the `diff -u /src/artifacts.lock /tmp/artifacts.lock` runtime check in the Dockerfile.
7. Run the double-build reproducibility check above, and commit the Dockerfile and the new `artifacts.lock` only after it prints `reproducible`.

## Updating edk2 or a patch

1. For an edk2 update, set `commit` in `source.lock` to the new edk2 commit and keep the patch applicable to it.
2. For a patch change, write the patch's new hash into `sha256` of its `[[patches]]` entry in `source.lock`:

   ```bash
   sha256sum ovmf/patches/amd-sev-grub.patch
   ```

3. Run the double-build reproducibility check above.
