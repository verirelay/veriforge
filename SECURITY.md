# Security policy

## Reporting a vulnerability

Report vulnerabilities through GitHub private vulnerability reporting: open the repository's **Security** tab and choose **Report a vulnerability**.

Do not open public issues for security problems.

## Scope

The build tooling in this repository:

- The initrd/vmlinuz build.
- The OVMF firmware build.
- The determinism guarantees of the produced artifacts.
- The Guest Manifest written into the initrd.

In particular, in scope:

- Verification of pinned inputs (kernel/module `.deb` SHA-256, CA bundle, edk2 commit and patch hashes, dpkg closure baseline).
- Anything that could silently alter the initrd or firmware bytes, or defeat the `dirty == false` release assertion.
- Injection of unintended content into `/etc/verirelay/manifest.json`.

Out of scope:

- The runtime behavior of the CVM booted from these artifacts (verirelay repo).
- VeriProv (veriprov repo).
- Vulnerabilities in third-party tools consumed at pinned versions.
