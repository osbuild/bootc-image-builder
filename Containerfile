FROM registry.fedoraproject.org/fedora:39 AS builder
RUN dnf install -y git-core golang gpgme-devel libassuan-devel
COPY build.sh .
RUN ./build.sh

FROM registry.fedoraproject.org/fedora:39
RUN dnf install -y osbuild osbuild-ostree && dnf clean all
COPY --from=builder images/osbuild-deploy-container /usr/bin/osbuild-deploy-container
COPY prepare.sh entrypoint.sh /
COPY --from=builder images/dnf-json .

ENTRYPOINT ["/entrypoint.sh"]
VOLUME /output
VOLUME /store
VOLUME /rpmmd

