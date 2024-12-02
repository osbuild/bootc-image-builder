# This Containerfile is targeted to a specialized path where you are changing
# pure Go code in this project and you're building on the host system with
# a compatible userspace (e.g. c9s or fedora).  We just take the already
# built binary and inject it on top of the main upstream container image.
#
# Crucially, using this flow imposes no constraints on your build setup,
# so that adding e.g. `replace github.com/osbuild/images => ../images`
# will Just Work.
#
# To use this, do e.g.:
#
# make && podman build --no-cache -t localhost/bib -v ./bin:/srcbin -f devel/Containerfile.hack .
#
# (The use of the explicit bind mount here is to bypass .dockerignore)
FROM quay.io/centos-bootc/bootc-image-builder:latest
RUN install /srcbin/bootc-image-builder /usr/bin
