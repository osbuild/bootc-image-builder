FROM registry.fedoraproject.org/fedora:39 AS builder
RUN dnf install -y git-core golang gpgme-devel libassuan-devel && mkdir -p /build/bib
COPY bib/go.mod bib/go.sum /build/bib
RUN cd /build/bib && go mod download
COPY build.sh /build
COPY bib /build/bib
WORKDIR /build
RUN ./build.sh

FROM registry.fedoraproject.org/fedora:39
# Install newer osbuild to fix the loop bug, see
# - https://github.com/osbuild/bootc-image-builder/issues/7
# - https://github.com/osbuild/bootc-image-builder/issues/9
# - https://github.com/osbuild/osbuild/pull/1468
COPY ./group_osbuild-osbuild-fedora-39.repo /etc/yum.repos.d/
RUN dnf install -y osbuild osbuild-ostree osbuild-depsolve-dnf && dnf clean all
COPY --from=builder /build/bin/bootc-image-builder /usr/bin/bootc-image-builder
COPY entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
VOLUME /output
VOLUME /store
VOLUME /rpmmd

