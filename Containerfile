FROM quay.io/centos/centos:stream10

# Konflux mounts in /etc/pki/entitlement instead of /run/secrets.
# This is not how we intended bib to work, but it works if subscription-manager is in bib.
# Include it temporarily, before we find a better long-term solution.
# See https://github.com/konflux-ci/build-definitions/blob/f3ac40bbc0230eccb8d98a4d54dabd55a4943c5d/task/build-vm-image/0.1/build-vm-image.yaml#L198
RUN dnf install -y \
    image-builder subscription-manager && \
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
