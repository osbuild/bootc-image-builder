FROM registry.access.redhat.com/ubi9/ubi:latest AS builder
RUN dnf install -y git-core golang && mkdir -p /build/bib
COPY bib/go.mod bib/go.sum /build/bib
RUN cd /build/bib && go mod download
COPY build.sh /build
COPY bib /build/bib
WORKDIR /build
RUN ./build.sh

FROM quay.io/centos/centos:stream9
# FROM registry.access.redhat.com/ubi9/ubi:latest
RUN dnf install -y osbuild osbuild-ostree  && dnf clean all
COPY --from=builder /build/bin/bootc-image-builder /usr/bin/bootc-image-builder
COPY entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
VOLUME /output
VOLUME /store
VOLUME /rpmmd

