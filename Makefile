PREFIX ?= /usr
DESTDIR ?=

.PHONY: all
all: bin/osbuildbootc

src:=$(shell find src -maxdepth 1 -type f -executable -print)
GOARCH:=$(shell uname -m)
ifeq ($(GOARCH),x86_64)
        GOARCH="amd64"
else ifeq ($(GOARCH),aarch64)
        GOARCH="arm64"
endif

.PHONY: bin/
bin/osbuildbootc:
	cd cmd && go build -mod vendor -o ../$@

.PHONY: check
check:
	(cd cmd && go test -mod=vendor)
	go test -mod=vendor github.com/coreos/coreos-assembler/internal/pkg/bashexec
	go test -mod=vendor github.com/coreos/coreos-assembler/internal/pkg/cosash

.PHONY: clean
clean:
	rm -rfv bin

.PHONY: insatll
install:
	install -d $(DESTDIR)$(PREFIX)/lib/osbuildbootc
	install -D -t $(DESTDIR)$(PREFIX)/lib/osbuildbootc $$(find src/ -maxdepth 1 -type f)
	install -D -t $(DESTDIR)$(PREFIX)/bin bin/osbuildbootc

.PHONY: vendor
vendor:
	@go mod vendor
	@go mod tidy

