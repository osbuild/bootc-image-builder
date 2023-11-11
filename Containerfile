FROM registry.fedoraproject.org/fedora:39 as builder
COPY . /src
RUN dnf -y install golang make
RUN make install DESTDIR=/instroot

FROM registry.fedoraproject.org/fedora:39
COPY --from=builder /instroot /
RUN /usr/lib/osbuildbootc/installdeps.sh