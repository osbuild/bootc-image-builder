FROM registry.fedoraproject.org/fedora:39 AS builder
RUN dnf install -y git-core golang gpgme-devel libassuan-devel
COPY build.sh .
RUN ./build.sh

FROM registry.fedoraproject.org/fedora:39
# Install newer osbuild to fix the loop bug, see
# - https://github.com/osbuild/osbuild-deploy-container/issues/7
# - https://github.com/osbuild/osbuild-deploy-container/issues/9
# - https://github.com/osbuild/osbuild/pull/1468
COPY ./group_osbuild-osbuild-fedora-39.repo /etc/yum.repos.d/
RUN dnf install -y osbuild osbuild-ostree && dnf clean all
COPY --from=builder bin/osbuild-deploy-container /usr/bin/osbuild-deploy-container
COPY prepare.sh entrypoint.sh /
COPY --from=builder images/dnf-json .

ENTRYPOINT ["/entrypoint.sh"]
VOLUME /output
VOLUME /store
VOLUME /rpmmd

