FROM ubuntu:26.04@sha256:19fcd1bf268f02b60aca3bd30bd8a6d9a11759a430e32921833e65f16d87348a AS builder

SHELL ["/bin/bash", "-euo", "pipefail", "-c"]

ARG UBUNTU_CODENAME=resolute
ARG APT_SNAPSHOT=20261027T000000Z

ENV DEBIAN_FRONTEND=noninteractive \
    TZ=UTC \
    LC_ALL=C.UTF-8 \
    LANG=C.UTF-8 \
    PYTHONHASHSEED=0

COPY ovmf/artifacts.lock /src/artifacts.lock

RUN cat > /etc/apt/sources.list.d/ubuntu.sources <<EOF
Types: deb
URIs: http://archive.ubuntu.com/ubuntu/
Suites: ${UBUNTU_CODENAME} ${UBUNTU_CODENAME}-updates ${UBUNTU_CODENAME}-backports
Components: main universe restricted multiverse
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg
Snapshot: ${APT_SNAPSHOT}

Types: deb
URIs: http://security.ubuntu.com/ubuntu/
Suites: ${UBUNTU_CODENAME}-security
Components: main universe restricted multiverse
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg
Snapshot: ${APT_SNAPSHOT}
EOF

RUN printf '%s\n' \
      'Acquire::Check-Valid-Until "false";' \
      'Acquire::Retries "5";' \
      'Acquire::https::Verify-Peer "false";' \
      'Acquire::https::Verify-Host "false";' \
    > /etc/apt/apt.conf.d/99snapshot && \
    apt-get update && \
    apt-get install -y --no-install-recommends --no-upgrade \
      ca-certificates=20260816ubuntu0.26.04.1 && \
    rm -rf /var/lib/apt/lists/*

# Snapshot is an APT deb822 field: it keeps the standard Ubuntu URIs while APT
# selects a fixed point in time. The full dpkg closure is verified right after
# apt install so a drift fails before the long build starts.
RUN cat > /etc/apt/sources.list.d/ubuntu.sources <<EOF
Types: deb
URIs: https://archive.ubuntu.com/ubuntu/
Suites: ${UBUNTU_CODENAME} ${UBUNTU_CODENAME}-updates ${UBUNTU_CODENAME}-backports
Components: main universe restricted multiverse
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg
Snapshot: ${APT_SNAPSHOT}

Types: deb
URIs: https://security.ubuntu.com/ubuntu/
Suites: ${UBUNTU_CODENAME}-security
Components: main universe restricted multiverse
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg
Snapshot: ${APT_SNAPSHOT}
EOF

RUN printf '%s\n' \
      'Acquire::Check-Valid-Until "false";' \
      'Acquire::Retries "5";' \
    > /etc/apt/apt.conf.d/99snapshot && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
      build-essential=12.12ubuntu2.26.04.2 \
      uuid-dev=2.41.3-3ubuntu2.2 \
      acpica-tools=20251212-1 \
      git=1:2.53.0-1ubuntu1.1 \
      nasm=3.01-1 \
      python3=3.14.3-0ubuntu2 \
      python-is-python3=3.13.3-1+build1 \
      mtools=4.0.49-1 \
      dosfstools=4.2-1.2build1 \
      grub-efi-amd64-bin=2.14-2ubuntu1 && \
    dpkg-query -W -f='${binary:Package}=${Version}\n' | sort > /tmp/artifacts.lock && \
    diff -u /src/artifacts.lock /tmp/artifacts.lock && \
    install -d /out && \
    cp /tmp/artifacts.lock /out/artifacts.lock && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

COPY ovmf/source.lock /src/source.lock
COPY ovmf/patches /src/patches
COPY ovmf/build.sh /src/build.sh

RUN bash /src/build.sh /out

FROM scratch AS export
COPY --from=builder /out/ /
