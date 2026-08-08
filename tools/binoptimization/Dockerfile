# syntax=docker/dockerfile:1.7
#
# Reusable binary-optimization toolkit for Ubuntu/Debian builders.
# Ships UPX + strip (binutils) so app Dockerfiles can COPY --from this
# image instead of apt-installing tools in every build stage.
#
# Multi-arch tag (same name for amd64 + arm64):
#   izetmolla/binoptimization:latest
#   izetmolla/containerws:binoptimization   (alias)
#
# Build / push (from this directory) — preferred:
#   ./build.sh                              # linux/amd64 + linux/arm64 → push
#   ./build.sh --platform linux/arm64       # arm64 only (DGX Spark)
#   ./build.sh --no-push                    # host arch, load locally
#
# Or raw buildx:
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     -t izetmolla/binoptimization:latest \
#     -t izetmolla/containerws:binoptimization \
#     --push .
#
# Layout:
#   /opt/binoptimization/bin/upx
#   /opt/binoptimization/bin/strip   (symlink → GNU strip)
#   /opt/binoptimization/bin/optimize
#   /usr/local/bin/{upx,strip,optimize}  (convenience symlinks)

FROM debian:bookworm-slim

ARG TARGETARCH
ARG UPX_VERSION=5.2.0

LABEL org.opencontainers.image.title="binoptimization" \
      org.opencontainers.image.description="UPX + strip toolkit for compressing Go/static binaries" \
      org.opencontainers.image.source="https://github.com/izetmolla/containerws"

ENV DEBIAN_FRONTEND=noninteractive \
    PATH="/opt/binoptimization/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

RUN set -eux; \
    apt-get update; \
    apt-get install --no-install-recommends -y \
      binutils \
      ca-certificates \
      wget \
      xz-utils; \
    arch="${TARGETARCH}"; \
    if [ -z "${arch}" ]; then \
      case "$(uname -m)" in \
        x86_64) arch=amd64 ;; \
        aarch64) arch=arm64 ;; \
        armv7l) arch=arm ;; \
        *) echo "unsupported arch $(uname -m)" >&2; exit 1 ;; \
      esac; \
    fi; \
    case "${arch}" in \
      amd64) upx_arch=amd64 ;; \
      arm64) upx_arch=arm64 ;; \
      arm) upx_arch=arm ;; \
      *) echo "unsupported TARGETARCH=${arch}" >&2; exit 1 ;; \
    esac; \
    wget -qO /tmp/upx.tar.xz \
      "https://github.com/upx/upx/releases/download/v${UPX_VERSION}/upx-${UPX_VERSION}-${upx_arch}_linux.tar.xz"; \
    tar -xJf /tmp/upx.tar.xz -C /tmp; \
    mkdir -p /opt/binoptimization/bin; \
    mv "/tmp/upx-${UPX_VERSION}-${upx_arch}_linux/upx" /opt/binoptimization/bin/upx; \
    chmod +x /opt/binoptimization/bin/upx; \
    ln -sf "$(command -v strip)" /opt/binoptimization/bin/strip; \
    ln -sf /opt/binoptimization/bin/upx /usr/local/bin/upx; \
    ln -sf /opt/binoptimization/bin/strip /usr/local/bin/strip; \
    rm -rf /tmp/upx.tar.xz "/tmp/upx-${UPX_VERSION}-${upx_arch}_linux" /var/lib/apt/lists/*

COPY optimize /opt/binoptimization/bin/optimize
RUN chmod +x /opt/binoptimization/bin/optimize && \
    ln -sf /opt/binoptimization/bin/optimize /usr/local/bin/optimize && \
    upx --version && \
    strip --version | head -1 && \
    optimize --help

# Default: show help (this image is a toolkit, not a long-running service)
ENTRYPOINT ["/opt/binoptimization/bin/optimize"]
CMD ["--help"]
