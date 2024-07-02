# We build osbuild from our pinned git submodule because
# of API instability between the vendored osbuild/images Go
# code and it.  This way we can update them both as a transactional unit.
# https://github.com/osbuild/bootc-image-builder/issues/376
FROM registry.fedoraproject.org/fedora:40 AS osbuild-builder
RUN dnf install -y rpm-build dnf-plugins-core git-core make
COPY . /src
WORKDIR /src/osbuild
RUN dnf builddep -y osbuild.spec
RUN make rpm

FROM registry.fedoraproject.org/fedora:40 AS bib-builder
RUN dnf install -y git-core golang gpgme-devel libassuan-devel && mkdir -p /build/bib
COPY bib/go.mod bib/go.sum /build/bib/
ARG GOPROXY=https://proxy.golang.org,direct
RUN go env -w GOPROXY=$GOPROXY
RUN cd /build/bib && go mod download
COPY package-requires.txt /build
COPY build.sh /build
COPY bib /build/bib
WORKDIR /build
RUN ./build.sh

FROM registry.fedoraproject.org/fedora:40
# Bind mount the two build sources, and install them.  We
# do this as a single layer to avoid having a distinct layer
# where the RPMs are redundantly captured.
RUN --mount=type=bind,from=osbuild-builder,target=/osbuild --mount=type=bind,from=bib-builder,target=/bib \
    dnf install -y /osbuild/src/osbuild/rpmbuild/RPMS/*/*.rpm && \
    grep -vE '^#' /bib/build/package-requires.txt | xargs dnf install -y && rm -f package-requires.txt && dnf clean all && \
    install /bib/build/bin/bootc-image-builder /usr/bin
COPY entrypoint.sh /
COPY bib/data /usr/share/bootc-image-builder

ENTRYPOINT ["/entrypoint.sh"]
VOLUME /output
WORKDIR /output
VOLUME /store
VOLUME /rpmmd
VOLUME /var/lib/containers/storage

LABEL description="This tools allows to build and deploy disk-images from bootc container inputs."
LABEL io.k8s.description="This tools allows to build and deploy disk-images from bootc container inputs."
LABEL io.k8s.display-name="Bootc Image Builder"
LABEL io.openshift.tags="base fedora40"
LABEL summary="A container to create disk-images from bootc container inputs"
