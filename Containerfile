FROM quay.io/fedora/fedora:39 as builder
RUN dnf -y install golang make
COPY . /src
RUN cd /src && make && make install DESTDIR=/instroot

FROM quay.io/fedora/fedora:39
COPY --from=builder /instroot /
RUN /usr/lib/osbuildbootc/installdeps.sh
