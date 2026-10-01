FROM quay.io/centos/centos:stream10

RUN dnf install -y \
    image-builder && \
    dnf clean all && \
    ln /usr/bin/image-builder /usr/bin/bootc-image-builder

ENTRYPOINT ["/usr/bin/bootc-image-builder"]
VOLUME /output
WORKDIR /output
VOLUME /store
VOLUME /rpmmd
VOLUME /var/lib/containers/storage

ENTRYPOINT ["/usr/bin/bootc-image-builder"]
VOLUME /output
WORKDIR /output
VOLUME /store
VOLUME /rpmmd
VOLUME /var/lib/containers/storage

LABEL description="This tools allows to build and deploy disk-images from bootc container inputs."
LABEL io.k8s.description="This tools allows to build and deploy disk-images from bootc container inputs."
LABEL io.k8s.display-name="Bootc Image Builder"
LABEL io.openshift.tags="base centos-stream-10"
LABEL summary="A container to create disk-images from bootc container inputs"
